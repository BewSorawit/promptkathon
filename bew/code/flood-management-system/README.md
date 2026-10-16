# เฝ้าระวังระดับน้ำไทย

ระบบต้นแบบสำหรับดูระดับน้ำที่สถานีตรวจวัดทั่วประเทศไทย แผนที่แสดงข้อมูลล่าสุดจาก ThaiWater และรายละเอียดแต่ละสถานีแสดงค่าที่วัดได้ กราฟย้อนหลัง และคำคาดการณ์ 6 ชั่วโมงเมื่อข้อมูลผ่านเกณฑ์ ระบบนี้ไม่ได้คำนวณขอบเขตพื้นที่น้ำท่วมและไม่ใช่ประกาศเตือนภัยทางการ

## เริ่มใช้งาน

ต้องมี Docker และ Docker Compose จากนั้นรันที่โฟลเดอร์โครงการ

```sh
docker compose up -d --build
```

เปิด [หน้าเว็บ](http://localhost:5173) หรือ [API](http://localhost:8080/healthz) ตัว API จะดึงข้อมูลล่าสุดเมื่อเริ่มทำงานและทุก 10 นาที เมื่อเปิดรายละเอียดสถานี ระบบจะดึงกราฟย้อนหลัง 7 วันจาก ThaiWater และพักผลดึงข้อมูลไว้ 10 นาที ฐานข้อมูลเก็บไว้ใน Docker volume ส่วนงานคาดการณ์ตรวจข้อมูลทุก 10 นาที

```sh
docker compose ps
docker compose logs -f api forecast
```

## ข้อมูลย้อนหลังและคำคาดการณ์

ข้อมูลย้อนหลังของ สสน. อยู่ใน [ชุดข้อมูลระดับน้ำ](https://data.go.th/en/dataset/water-level?is_fullscreen=1) และแยกไฟล์ CSV ตามเดือนกับรหัสสถานี ตัวนำเข้ารองรับ `station_code,measure_datetime,water_level,quality_flag` และชื่อคอลัมน์แบบ `station_id,observed_at,level_msl` โดยข้ามค่า `null`, `-999`, `9999`, `999999`, เวลาไม่ถูกต้อง และแถวที่ `quality_flag` ไม่ใช่ `N` รหัสสถานีดูได้จากฟิลด์ `code` ของ API

ตัวอย่างนำเข้าข้อมูลของสถานี `THA006` สองเดือน ไฟล์เดือนละหนึ่งสถานี

```sh
docker compose run --rm --no-deps --entrypoint /import-csv api 'https://tiservice.hii.or.th/opendata/data_catalog/water_level/2026/202608/THA006.csv'
docker compose run --rm --no-deps --entrypoint /import-csv api 'https://tiservice.hii.or.th/opendata/data_catalog/water_level/2026/202609/THA006.csv'
docker compose exec forecast python -c 'from worker import run_once; run_once()'
```

ใส่พาธไฟล์ CSV ในเครื่องแทน URL ได้ โดย mount โฟลเดอร์ไฟล์เข้า container ก่อนนำเข้า การรันคำสั่งซ้ำจะไม่เพิ่มค่าที่เวลาเดิม

แบบจำลอง Ridge regression ฝึกแยกตามสถานีจากช่วงข้อมูลต่อเนื่องอย่างน้อย 30 วันและมีข้อมูลอย่างน้อย 80% ใช้ระดับน้ำก่อนหน้า 0, 1, 3 และ 6 ชั่วโมงเพื่อคาดการณ์ส่วนต่างของระดับน้ำในอีก 6 ชั่วโมง ตรวจผลกับข้อมูลช่วงท้ายตามลำดับเวลาเทียบกับวิธีใช้ค่าล่าสุดคงเดิม หากแบบจำลองไม่ดีกว่าวิธีพื้นฐาน สถานีนั้นไม่มีคำคาดการณ์ แม้แบบจำลองผ่านเกณฑ์แล้ว ต้องมีข้อมูลล่าสุดย้อนหลัง 6 ชั่วโมงสำหรับสร้างคำคาดการณ์ปัจจุบัน หากค่าล่าสุดเก่าเกิน 30 นาที API จะซ่อนคำคาดการณ์

## API

| Endpoint | ผลลัพธ์ |
| --- | --- |
| `GET /healthz` | สถานะฐานข้อมูล |
| `GET /api/v1/stations?province=สุพรรณบุรี` | สถานีล่าสุดและสถานะข้อมูลเก่า |
| `GET /api/v1/stations/{id}` | รายละเอียดสถานี |
| `GET /api/v1/stations/{id}/observations?hours=48` | ระดับน้ำย้อนหลัง 1 ถึง 168 ชั่วโมง |
| `GET /api/v1/stations/{id}/forecast` | ค่าคาดการณ์หรือเหตุผลที่ยังไม่มี |
| `GET /api/v1/source-status` | เวลาดึงข้อมูลและข้อผิดพลาดล่าสุด |

หน่วยระดับน้ำคือเมตรเทียบระดับน้ำทะเลปานกลาง (ม.รทก.) เวลาใน API เป็น UTC และหน้าเว็บแสดงเวลาไทย ค่า `situation_level` บนแผนที่มาจากผู้ให้ข้อมูล ส่วนค่าคาดการณ์สร้างโดยโครงการนี้

## ตรวจการทำงาน

```sh
cd backend && go test ./... && go vet ./...
cd ../front-end && npm ci && npm run build
cd .. && docker compose exec forecast python -m unittest -v test_model
```

ข้อมูลย้อนหลังใช้สิทธิ์ [CC BY-NC](https://data.go.th/en/dataset/water-level?is_fullscreen=1) ระบบจึงตั้งใจให้ใช้เป็นต้นแบบที่ไม่แสวงกำไร หน้าแผนที่แสดงเครดิต OpenStreetMap ตาม [นโยบาย tile](https://operations.osmfoundation.org/policies/tiles/) ก่อนเผยแพร่จริงต้องตรวจสิทธิ์ข้อมูลและเลือกบริการ tile ที่รองรับปริมาณผู้ใช้
