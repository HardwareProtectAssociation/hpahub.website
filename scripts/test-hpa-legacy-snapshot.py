#!/usr/bin/env python3
"""Focused synthetic tests for the legacy snapshot command."""

import hashlib
import json
import os
from pathlib import Path
import sqlite3
import stat
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("hpa-legacy-snapshot.py")


class SnapshotTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.gpu = self.root / "gpu?#.db"
        self.auth = self.root / "users.db"
        self.warehouse = self.root / "warehouse"
        (self.warehouse / "projects" / "空目录").mkdir(parents=True)
        self.output = self.root / "snapshot"
        # Keep the WAL connection open so the snapshot must include WAL data.
        self.db = sqlite3.connect(self.gpu)
        self.addCleanup(self.db.close)
        self.db.execute("PRAGMA journal_mode=WAL")
        self.db.execute('CREATE TABLE "gpu table" (value TEXT)')
        self.db.execute('INSERT INTO "gpu table" VALUES (?)', ("显卡",))
        self.db.commit()
        with sqlite3.connect(self.auth) as db:
            db.execute("CREATE TABLE users (password_hash TEXT)")
            db.executemany("INSERT INTO users VALUES (?)", [("scrypt:32768:8:1$salt$hash",), ("other$salt$hash",)])

    def run_snapshot(self, success=True):
        result = subprocess.run([sys.executable, str(SCRIPT), "--gpu-db", str(self.gpu),
                                 "--auth-db", str(self.auth), "--warehouse-root", str(self.warehouse),
                                 "--output", str(self.output)], capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, success, result.stderr)
        return result

    def test_snapshot_wal_manifest_and_permissions(self):
        project = self.warehouse / "projects" / "项目"
        project.mkdir()
        (project / ".meta.json").write_bytes(b"metadata")
        (project / "photo 1.png").write_bytes(b"image")
        (self.warehouse / "storage_config.json").write_bytes(b"{}")
        self.run_snapshot()
        manifest = json.loads((self.output / "manifest.json").read_text())
        self.assertEqual(manifest["gpu_tables"]["gpu table"], 1)
        self.assertEqual(manifest["accounts"], {"count": 2, "hash_algorithms": {"scrypt:32768:8:1": 1, "other": 1}})
        self.assertTrue(manifest["storage_config_present"])
        entries = {entry["path"]: entry for entry in manifest["warehouse_files"]}
        self.assertEqual(set(entries), {"projects/项目/.meta.json", "projects/项目/photo 1.png", "storage_config.json"})
        for path, entry in entries.items():
            data = (self.output / "warehouse" / path).read_bytes()
            self.assertEqual((entry["size"], entry["sha256"]), (len(data), hashlib.sha256(data).hexdigest()))
        self.assertTrue((self.output / "warehouse/projects/空目录").is_dir())
        with sqlite3.connect(self.output / "gpu.db") as db:
            self.assertEqual(db.execute('SELECT value FROM "gpu table"').fetchone(), ("显卡",))
        for path in [self.output, *(p for p in self.output.rglob("*") if p.is_dir())]:
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o700)
        for path in (p for p in self.output.rglob("*") if p.is_file()):
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        self.run_snapshot(False)
        self.assertEqual(json.loads((self.output / "manifest.json").read_text()), manifest)

    def test_absent_config_and_empty_projects(self):
        self.run_snapshot()
        manifest = json.loads((self.output / "manifest.json").read_text())
        self.assertFalse(manifest["storage_config_present"])
        self.assertEqual(manifest["warehouse_files"], [])
        self.assertTrue((self.output / "warehouse/projects/空目录").is_dir())

    def test_reject_unsafe_or_missing_sources_and_cleanup(self):
        projects = self.warehouse / "projects"
        unsafe = projects / "unsafe"
        for kind in ("symlink", "fifo"):
            if kind == "symlink":
                unsafe.symlink_to(self.auth)
            else:
                os.mkfifo(unsafe)
            self.run_snapshot(False)
            self.assertFalse(self.output.exists())
            self.assertEqual(list(self.root.glob(".snapshot.*")), [])
            unsafe.unlink()
        (self.warehouse / "storage_config.json").symlink_to(self.auth)
        self.run_snapshot(False)
        (self.warehouse / "storage_config.json").unlink()
        projects.rename(self.warehouse / "missing")
        self.run_snapshot(False)
        self.assertFalse(self.output.exists())
        self.assertEqual(list(self.root.glob(".snapshot.*")), [])

    def test_invalid_database_and_destination_symlink(self):
        self.auth.write_bytes(b"not sqlite")
        self.run_snapshot(False)
        self.assertFalse(self.output.exists())
        self.output.symlink_to(self.root / "missing")
        self.run_snapshot(False)
        self.assertTrue(self.output.is_symlink())
        self.assertEqual(list(self.root.glob(".snapshot.*")), [])


if __name__ == "__main__":
    unittest.main()
