import json
import logging
import os
import time
from datetime import datetime, timedelta, timezone

import psycopg
from psycopg.rows import dict_row

from model import grid_from_rows, predict, train

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
DATABASE_URL = os.environ["DATABASE_URL"]


def process_station(conn, station_id, now):
    rows = conn.execute(
        "SELECT observed_at, level_msl FROM observations WHERE station_id=%s AND observed_at >= %s ORDER BY observed_at",
        (station_id, now - timedelta(days=60)),
    ).fetchall()
    if not rows:
        return
    grid = grid_from_rows([(row["observed_at"], row["level_msl"]) for row in rows])
    latest_at = rows[-1]["observed_at"]
    existing = conn.execute(
        "SELECT id, trained_at, status, coefficients, intercept, residual_p90 FROM model_runs WHERE station_id=%s ORDER BY trained_at DESC LIMIT 1",
        (station_id,),
    ).fetchone()
    if existing is None or now - existing["trained_at"] >= timedelta(hours=24):
        result = train(grid)
        if result is not None:
            existing = conn.execute(
                """INSERT INTO model_runs (station_id,sample_count,validation_mae,baseline_mae,residual_p90,status,coefficients,intercept)
                VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
                RETURNING id,trained_at,status,coefficients,intercept,residual_p90""",
                (station_id, result["sample_count"], result["validation_mae"], result["baseline_mae"],
                 result["residual_p90"], result["status"], json.dumps(result["coefficients"]), result["intercept"]),
            ).fetchone()
    if not existing or existing["status"] != "passed" or now - latest_at > timedelta(minutes=30):
        return
    origin = latest_at.replace(minute=latest_at.minute // 10 * 10, second=0, microsecond=0)
    result = predict(grid, origin, existing)
    if result is None:
        return
    value, lower, upper = result
    conn.execute(
        """INSERT INTO forecasts (station_id,origin_at,target_at,predicted_msl,lower_msl,upper_msl,model_run_id)
        VALUES (%s,%s,%s,%s,%s,%s,%s) ON CONFLICT DO NOTHING""",
        (station_id, origin, origin + timedelta(hours=6), value, lower, upper, existing["id"]),
    )


def run_once():
    now = datetime.now(timezone.utc)
    with psycopg.connect(DATABASE_URL, row_factory=dict_row) as conn:
        stations = conn.execute("SELECT id FROM stations ORDER BY id").fetchall()
        for station in stations:
            try:
                with conn.transaction():
                    process_station(conn, station["id"], now)
            except Exception:
                logging.exception("forecast failed for station %s", station["id"])
        logging.info("checked %d stations", len(stations))


if __name__ == "__main__":
    while True:
        try:
            run_once()
        except Exception:
            logging.exception("forecast cycle failed")
        time.sleep(600)
