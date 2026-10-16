from datetime import timedelta
import numpy as np
from sklearn.linear_model import Ridge
from sklearn.metrics import mean_absolute_error

SLOT = timedelta(minutes=10)
LAGS = (0, 6, 18, 36)
HORIZON = 36
MIN_DAYS = 30


def grid_from_rows(rows):
    grid = {}
    for at, level in rows:
        slot = at.replace(minute=at.minute // 10 * 10, second=0, microsecond=0)
        if abs((at - slot).total_seconds()) <= 120:
            grid[slot] = float(level)
    return grid


def features(grid, origin):
    values = [grid.get(origin - SLOT * lag) for lag in LAGS]
    if any(value is None for value in values):
        return None
    return [*values, values[0] - values[1], values[1] - values[2]]


def training_pairs(grid):
    if not grid:
        return np.empty((0, 6)), np.empty(0)
    times = sorted(grid)
    segments = []
    current = [times[0]]
    for at in times[1:]:
        if at - current[-1] > timedelta(days=1):
            segments.append(current)
            current = []
        current.append(at)
    segments.append(current)
    eligible = [segment for segment in segments if (segment[-1] - segment[0]).days >= MIN_DAYS and len(segment) / (int((segment[-1] - segment[0]) / SLOT) + 1) >= 0.8]
    if not eligible:
        return np.empty((0, 6)), np.empty(0)
    times = eligible[-1]
    x, y = [], []
    for origin in times:
        if origin.minute != 0:
            continue
        target = grid.get(origin + SLOT * HORIZON)
        row = features(grid, origin)
        if row is not None and target is not None:
            x.append(row)
            y.append(target)
    return np.asarray(x, dtype=float).reshape(-1, 6), np.asarray(y, dtype=float)


def train(grid):
    x, y = training_pairs(grid)
    if len(y) < 24 * MIN_DAYS * 0.8:
        return None
    split = int(len(y) * 0.8)
    model = Ridge(alpha=1.0)
    delta = y - x[:, 0]
    model.fit(x[:split], delta[:split])
    predicted = x[split:, 0] + model.predict(x[split:])
    mae = mean_absolute_error(y[split:], predicted)
    baseline = mean_absolute_error(y[split:], x[split:, 0])
    residual_p90 = float(np.percentile(np.abs(y[split:] - predicted), 90))
    passed = bool(mae < baseline and np.isfinite(residual_p90))
    if passed:
        model.fit(x, delta)
    return {
        "sample_count": len(y), "validation_mae": float(mae),
        "baseline_mae": float(baseline), "residual_p90": residual_p90,
        "status": "passed" if passed else "rejected",
        "coefficients": model.coef_.tolist() if passed else None,
        "intercept": float(model.intercept_) if passed else None,
    }


def predict(grid, origin, model_run):
    row = features(grid, origin)
    if row is None:
        return None
    level = float(row[0] + np.dot(np.asarray(model_run["coefficients"]), np.asarray(row)) + model_run["intercept"])
    if not np.isfinite(level):
        return None
    spread = model_run["residual_p90"]
    return level, level - spread, level + spread
