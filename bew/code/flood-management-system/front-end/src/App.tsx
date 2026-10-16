import { useEffect, useMemo, useRef, useState } from 'react'
import * as maplibregl from 'maplibre-gl'
import type { Map, MapMouseEvent } from 'maplibre-gl'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
import { api, type ForecastResult, type Observation, type Station } from './api'

maplibregl.setWorkerUrl(workerUrl)

type Detail = { kind: 'loading' } | { kind: 'ready'; history: Observation[]; forecast: ForecastResult | null; status: string } | { kind: 'error'; message: string }

function dateTime(value: string | null) {
  return value ? new Intl.DateTimeFormat('th-TH', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Bangkok' }).format(new Date(value)) : 'ไม่มีข้อมูล'
}
function statusText(station: Station) {
  if (station.is_stale) return 'ข้อมูลเก่า'
  switch (station.situation_level) {
    case 5: return 'ล้นตลิ่ง'
    case 4: return 'น้ำมาก'
    case 3: return 'ปกติ'
    case 2: return 'น้ำน้อย'
    case 1: return 'น้ำน้อยวิกฤต'
    default: return 'ไม่ระบุสถานะ'
  }
}
function statusColor(station: Station) {
  if (station.is_stale) return '#89969e'
  return ({ 1: '#9bc9dc', 2: '#4b9fc4', 3: '#37a879', 4: '#f0a63b', 5: '#dc6159' } as Record<number, string>)[station.situation_level ?? 0] ?? '#89969e'
}
function Trend({ observations, forecast }: { observations: Observation[]; forecast: ForecastResult | null }) {
  const points = observations.slice(-145)
  if (points.length < 2) return <p className="empty">ข้อมูลย้อนหลังยังไม่พอสำหรับกราฟ</p>
  const values = points.map(p => p.level_msl)
  if (forecast) values.push(forecast.lower_msl, forecast.upper_msl)
  const min = Math.min(...values) - 0.1
  const max = Math.max(...values) + 0.1
  const x = (i: number) => 10 + i * 300 / (points.length - 1 + (forecast ? 18 : 0))
  const y = (v: number) => 100 - (v - min) / (max - min) * 85
  const path = points.map((p, i) => `${i ? 'L' : 'M'} ${x(i)} ${y(p.level_msl)}`).join(' ')
  return <div className="chart"><svg viewBox="0 0 320 115" role="img" aria-label="กราฟระดับน้ำย้อนหลังและคาดการณ์"><path d={path} fill="none" stroke="#187d91" strokeWidth="2.5" strokeLinejoin="round" />{forecast && <><line x1={x(points.length - 1)} y1={y(points.at(-1)!.level_msl)} x2={x(points.length + 17)} y2={y(forecast.predicted_msl)} stroke="#d27042" strokeWidth="2" strokeDasharray="5 4" /><circle cx={x(points.length + 17)} cy={y(forecast.predicted_msl)} r="4" fill="#d27042" /></>}</svg><div className="chart-labels"><span>ย้อนหลังประมาณ 24 ชั่วโมง</span><span>{forecast ? 'อีก 6 ชั่วโมง' : 'ล่าสุด'}</span></div></div>
}
function MapView({ stations, province, onSelect }: { stations: Station[]; province: string; onSelect: (id: number) => void }) {
  const container = useRef<HTMLDivElement>(null)
  const map = useRef<Map | null>(null)
  const onSelectRef = useRef(onSelect)
  const stationsRef = useRef(stations)
  onSelectRef.current = onSelect
  stationsRef.current = stations
  const updateStations = (instance: Map, values: Station[]) => {
    const source = instance.getSource('stations') as maplibregl.GeoJSONSource | undefined
    source?.setData({ type: 'FeatureCollection', features: values.map(s => ({ type: 'Feature', geometry: { type: 'Point', coordinates: [s.longitude, s.latitude] }, properties: { id: s.id, color: statusColor(s) } })) })
  }
  useEffect(() => {
    if (!container.current || map.current) return
    const instance = new maplibregl.Map({
      container: container.current,
      style: { version: 8, sources: { osm: { type: 'raster', tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'], tileSize: 256, attribution: '© OpenStreetMap contributors' } }, layers: [{ id: 'osm', type: 'raster', source: 'osm' }] },
      center: [101, 15.5], zoom: 5,
    })
    instance.addControl(new maplibregl.NavigationControl(), 'top-right')
    instance.on('load', () => {
      instance.addSource('stations', { type: 'geojson', data: { type: 'FeatureCollection', features: [] }, cluster: true, clusterRadius: 45, clusterMaxZoom: 10 })
      instance.addLayer({ id: 'clusters', type: 'circle', source: 'stations', filter: ['has', 'point_count'], paint: { 'circle-color': '#155a70', 'circle-radius': ['step', ['get', 'point_count'], 20, 20, 25, 100, 31], 'circle-stroke-color': '#fff', 'circle-stroke-width': 2 } })
      instance.addLayer({ id: 'cluster-count', type: 'symbol', source: 'stations', filter: ['has', 'point_count'], layout: { 'text-field': ['get', 'point_count_abbreviated'], 'text-size': 12 }, paint: { 'text-color': '#fff' } })
      instance.addLayer({ id: 'station-points', type: 'circle', source: 'stations', filter: ['!', ['has', 'point_count']], paint: { 'circle-color': ['get', 'color'], 'circle-radius': 8, 'circle-stroke-color': '#fff', 'circle-stroke-width': 2 } })
      updateStations(instance, stationsRef.current)
      instance.on('click', 'station-points', e => { const id = Number(e.features?.[0]?.properties?.id); if (id) onSelectRef.current(id) })
      instance.on('click', 'clusters', async (e: MapMouseEvent) => { const features = instance.queryRenderedFeatures(e.point, { layers: ['clusters'] }); const clusterId = features[0]?.properties?.cluster_id; const source = instance.getSource('stations') as maplibregl.GeoJSONSource; if (clusterId != null && features[0]?.geometry.type === 'Point') { const zoom = await source.getClusterExpansionZoom(clusterId); const coordinates = features[0].geometry.coordinates; instance.easeTo({ center: [coordinates[0], coordinates[1]], zoom }) } })
      instance.on('mouseenter', 'station-points', () => { instance.getCanvas().style.cursor = 'pointer' })
      instance.on('mouseleave', 'station-points', () => { instance.getCanvas().style.cursor = '' })
    })
    map.current = instance
    return () => { instance.remove(); map.current = null }
  }, [])
  useEffect(() => {
    const instance = map.current
    if (!instance) return
    updateStations(instance, stations)
  }, [stations])
  useEffect(() => {
    const instance = map.current
    if (!instance) return
    if (!province) { instance.flyTo({ center: [101, 15.5], zoom: 5 }); return }
    if (!stations.length) return
    const bounds = new maplibregl.LngLatBounds()
    stations.forEach(station => bounds.extend([station.longitude, station.latitude]))
    instance.fitBounds(bounds, { padding: 80, maxZoom: 10 })
  }, [province])
  return <div ref={container} className="map" aria-label="แผนที่สถานีวัดน้ำทั่วประเทศไทย" />
}

export default function App() {
  const [stations, setStations] = useState<Station[]>([])
  const [province, setProvince] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [detail, setDetail] = useState<Detail | null>(null)
  const [source, setSource] = useState<{ status: string; succeeded_at: string | null } | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    const load = async () => { try { const [stationsResult, sourceResult] = await Promise.all([api.stations(), api.source()]); if (active) { setStations(stationsResult.stations); setSource(sourceResult.sync); setError('') } } catch (e) { if (active) setError(e instanceof Error ? e.message : 'โหลดข้อมูลไม่สำเร็จ') } }
    void load(); const timer = window.setInterval(load, 60_000)
    return () => { active = false; window.clearInterval(timer) }
  }, [])
  useEffect(() => {
    if (selectedId == null) return
    let active = true
    setDetail({ kind: 'loading' })
    Promise.all([api.observations(selectedId), api.forecast(selectedId)]).then(([h, f]) => { if (active) setDetail({ kind: 'ready', history: h.observations, forecast: f.forecast, status: f.status }) }).catch(e => { if (active) setDetail({ kind: 'error', message: e instanceof Error ? e.message : 'โหลดรายละเอียดไม่สำเร็จ' }) })
    return () => { active = false }
  }, [selectedId, stations])
  const provinces = useMemo(() => [...new Set(stations.map(s => s.province).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'th')), [stations])
  const visible = useMemo(() => province ? stations.filter(s => s.province === province) : stations, [stations, province])
  const selected = stations.find(s => s.id === selectedId)
  const currentCount = visible.filter(s => !s.is_stale).length
  const highCount = visible.filter(s => !s.is_stale && (s.situation_level === 4 || s.situation_level === 5)).length
  return <div className="app">
    <header className="topbar"><div className="brand"><span className="brand-icon">≈</span><div><strong>เฝ้าระวังระดับน้ำไทย</strong><small>ข้อมูลสถานีวัดน้ำทั่วประเทศ</small></div></div><span className="prototype">ระบบต้นแบบ</span></header>
    <main className="layout"><aside className="sidebar"><div className="intro"><p className="eyebrow">ภาพรวมสถานการณ์</p><h1>ระดับน้ำ<br />ใกล้ตัวคุณ</h1><p>ติดตามค่าที่วัดได้จากสถานีทั่วประเทศ และดูแนวโน้มล่วงหน้า 6 ชั่วโมงในจุดที่มีข้อมูลเพียงพอ</p></div>
      <div className="summary"><div><strong>{visible.length}</strong><span>สถานีที่แสดง</span></div><div><strong>{currentCount}</strong><span>ข้อมูลล่าสุด</span></div><div><strong>{highCount}</strong><span>น้ำมาก/ล้นตลิ่ง</span></div></div>
      <label className="filter-label" htmlFor="province">เลือกจังหวัด</label><select id="province" value={province} onChange={e => { setProvince(e.target.value); setSelectedId(null) }}><option value="">ทุกจังหวัด</option>{provinces.map(p => <option key={p} value={p}>{p}</option>)}</select>
      {province && <div className="station-list"><h2>สถานีในจังหวัด</h2>{visible.map(station => <button key={station.id} onClick={() => setSelectedId(station.id)}><i style={{background:statusColor(station)}} /><span>{station.name}</span><small>{station.level_msl?.toFixed(2) ?? '—'} ม.</small></button>)}</div>}
      <div className="legend"><h2>สัญลักษณ์บนแผนที่</h2>{[['#9bc9dc', 'น้ำน้อยวิกฤต'], ['#4b9fc4', 'น้ำน้อย'], ['#37a879', 'ปกติ'], ['#f0a63b', 'น้ำมาก'], ['#dc6159', 'ล้นตลิ่ง'], ['#89969e', 'ข้อมูลเก่า/ไม่ระบุ']].map(([color, label]) => <div key={label}><i style={{ background: color }} />{label}</div>)}</div>
      <div className="source-box"><b>แหล่งข้อมูล</b><span>ThaiWater / สสน.</span><span>ซิงก์ล่าสุด {dateTime(source?.succeeded_at ?? null)}</span>{source?.status === 'error' && <em>ดึงข้อมูลรอบล่าสุดไม่สำเร็จ</em>}{error && <em>{error}</em>}</div>
      <p className="notice">สีแสดงสถานะที่สถานีวัด ไม่ได้แสดงขอบเขตพื้นที่น้ำท่วม ค่าคาดการณ์เป็นข้อมูลประกอบการติดตาม ไม่ใช่ประกาศเตือนภัยทางการ</p>
    </aside><section className="map-panel"><MapView stations={visible} province={province} onSelect={setSelectedId} /><div className="map-caption">คลิกจุดสถานีเพื่อดูระดับน้ำย้อนหลังและคำคาดการณ์</div></section>
    {selected && <aside className="detail"><button className="close" onClick={() => setSelectedId(null)} aria-label="ปิดรายละเอียด">×</button><p className="eyebrow">รายละเอียดสถานี</p><h2>{selected.name}</h2><p className="location">{selected.district && `${selected.district}, `}{selected.province}{selected.river && ` · ${selected.river}`}</p><span className="status-pill" style={{ color: statusColor(selected) }}>{statusText(selected)}</span><div className="reading"><strong>{selected.level_msl?.toFixed(2) ?? '—'}</strong><span>เมตร รทก.</span></div><p className="timestamp">วัดเมื่อ {dateTime(selected.observed_at)}</p>{selected.is_stale && <p className="warning">ข้อมูลเกิน 30 นาที โปรดตรวจสอบแหล่งข้อมูลก่อนใช้ตัดสินใจ</p>}
      <h3>การเปลี่ยนแปลงระดับน้ำ</h3>{detail?.kind === 'ready' ? <Trend observations={detail.history} forecast={detail.forecast} /> : <p className="empty">{detail?.kind === 'error' ? detail.message : 'กำลังโหลดข้อมูล...'}</p>}
      <div className="forecast-card"><span>คาดการณ์อีก 6 ชั่วโมง</span>{detail?.kind === 'ready' && detail.forecast && !selected.is_stale ? <><strong>{detail.forecast.predicted_msl.toFixed(2)} ม.รทก.</strong><small>ช่วงประมาณ {detail.forecast.lower_msl.toFixed(2)}–{detail.forecast.upper_msl.toFixed(2)} ม.</small><small>เป้าหมาย {dateTime(detail.forecast.target_at)}</small></> : <p>{detail?.kind === 'ready' && detail.status === 'stale' ? 'ข้อมูลล่าสุดเก่าเกินไป' : 'ยังไม่มีคำคาดการณ์ที่ผ่านเกณฑ์'}</p>}</div><p className="credit">ที่มา ThaiWater / สสน. · ค่าคาดการณ์จากแบบจำลองของโครงการ</p>
    </aside>}</main>
  </div>
}
