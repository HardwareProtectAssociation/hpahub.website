#!/usr/bin/env python3
"""Check that unsafe archives are rejected before Docker is invoked."""
import os
import pty
import subprocess
import tempfile
import zipfile
from pathlib import Path

restore = Path(__file__).resolve().with_name('local-restore.sh')
with tempfile.TemporaryDirectory() as directory:
    archive = Path(directory) / 'unsafe.zip'
    escaped = Path(directory).name + '-escaped'
    with zipfile.ZipFile(archive, 'w') as backup:
        backup.writestr('../' + escaped, 'invalid')
    master, slave = pty.openpty()
    try:
        result = subprocess.run([str(restore), str(archive)], stdin=slave,
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    finally:
        os.close(master)
        os.close(slave)
    assert result.returncode != 0 and '不安全' in result.stdout, result.stdout
    assert not (Path(directory).parent / escaped).exists()
print('Unsafe backup rejected')
