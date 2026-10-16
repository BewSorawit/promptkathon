import unittest
from datetime import datetime, timedelta, timezone

from model import grid_from_rows, predict, train, training_pairs


class ForecastModelTests(unittest.TestCase):
    def test_requires_history_and_contiguous_lags(self):
        start = datetime(2026, 1, 1, tzinfo=timezone.utc)
        grid = {start + timedelta(minutes=10 * i): 2.0 for i in range(100)}
        self.assertEqual(len(training_pairs(grid)[1]), 0)
        self.assertIsNone(train(grid))

    def test_trend_can_beat_persistence_on_later_holdout(self):
        start = datetime(2026, 1, 1, tzinfo=timezone.utc)
        rows = [(start + timedelta(minutes=10 * i), 2 + i * 0.0001) for i in range(35 * 24 * 6)]
        grid = grid_from_rows(rows)
        result = train(grid)
        self.assertIsNotNone(result)
        self.assertEqual(result["status"], "passed")
        self.assertLess(result["validation_mae"], result["baseline_mae"])
        point = predict(grid, rows[-1][0], result)
        self.assertIsNotNone(point)
        self.assertLess(point[1], point[0])
        self.assertGreater(point[2], point[0])

    def test_recent_isolated_reading_does_not_disqualify_archive(self):
        start = datetime(2026, 1, 1, tzinfo=timezone.utc)
        rows = [(start + timedelta(minutes=10 * i), 2 + i * 0.0001) for i in range(35 * 24 * 6)]
        rows.append((start + timedelta(days=60), 4.0))
        self.assertGreater(len(training_pairs(grid_from_rows(rows))[1]), 500)


if __name__ == "__main__":
    unittest.main()
