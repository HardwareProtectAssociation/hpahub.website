#!/usr/bin/env python3
"""Create a private, consistent snapshot of the legacy services' data."""

import argparse
import ctypes
from contextlib import closing
import errno
import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import stat
import sys
import tempfile


def reject_symlink_path(path):
    path = Path(os.path.abspath(path))
    current = Path(path.anchor)
    for part in path.parts[1:]:
        current /= part
        if stat.S_ISLNK(os.lstat(current).st_mode):
            raise ValueError(f"symlink not allowed: {current}")
    return path


def backup_database(source, destination):
    source = reject_symlink_path(source)
    if not stat.S_ISREG(os.stat(source, follow_symlinks=False).st_mode):
        raise ValueError(f"not a regular database file: {source}")
    # URI quoting is required for paths containing ? or #.
    with closing(sqlite3.connect(source.as_uri() + "?mode=ro", uri=True)) as src:
        with closing(sqlite3.connect(destination)) as dst:
            os.chmod(destination, 0o600)
            src.backup(dst)
            result = dst.execute("PRAGMA integrity_check").fetchall()
            if result != [("ok",)]:
                raise ValueError(f"database integrity check failed: {source}")


def copy_file(parent_fd, name, target):
    fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent_fd)
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise ValueError(f"not a regular file: {name}")
        digest = hashlib.sha256()
        size = 0
        with os.fdopen(fd, "rb") as source:
            fd = -1
            with open(target, "xb") as dest:
                os.chmod(target, 0o600)
                while chunk := source.read(1024 * 1024):
                    dest.write(chunk)
                    digest.update(chunk)
                    size += len(chunk)
        return {"path": target.as_posix(), "size": size, "sha256": digest.hexdigest()}
    finally:
        if fd != -1:
            os.close(fd)


def copy_tree(source_fd, target, root, entries):
    for name in sorted(os.listdir(source_fd)):
        mode = os.stat(name, dir_fd=source_fd, follow_symlinks=False).st_mode
        child = target / name
        if stat.S_ISDIR(mode):
            fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=source_fd)
            try:
                child.mkdir(mode=0o700)
                os.chmod(child, 0o700)
                copy_tree(fd, child, root, entries)
            finally:
                os.close(fd)
        elif stat.S_ISREG(mode):
            entry = copy_file(source_fd, name, child)
            entry["path"] = child.relative_to(root).as_posix()
            entries.append(entry)
        else:
            raise ValueError(f"symlink or special file not allowed: {child.relative_to(root)}")


def publish(source, destination):
    # renameat2 provides atomic no-replace semantics even if another process
    # creates the destination after the initial check.
    libc = ctypes.CDLL(None, use_errno=True)
    renameat2 = libc.renameat2
    renameat2.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
    renameat2.restype = ctypes.c_int
    if renameat2(-100, os.fsencode(source), -100, os.fsencode(destination), 1) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error), str(destination))


def snapshot(gpu_db, warehouse_root, auth_db, output):
    output = Path(os.path.abspath(output))
    parent = reject_symlink_path(output.parent)
    output = parent / output.name
    if os.path.lexists(output):
        raise FileExistsError(errno.EEXIST, os.strerror(errno.EEXIST), str(output))
    warehouse_root = reject_symlink_path(warehouse_root)
    root_fd = os.open(warehouse_root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    temp = None
    try:
        # Do not copy the snapshot into itself when output is inside projects.
        projects = reject_symlink_path(warehouse_root / "projects")
        if os.path.commonpath((str(projects), str(output))) == str(projects):
            raise ValueError("output cannot be inside warehouse projects")
        temp = Path(tempfile.mkdtemp(prefix=f".{output.name}.", dir=parent))
        os.chmod(temp, 0o700)
        backup_database(gpu_db, temp / "gpu.db")
        backup_database(auth_db, temp / "users.db")
        with closing(sqlite3.connect(temp / "gpu.db")) as db:
            tables = db.execute("SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name").fetchall()
            counts = {name: db.execute('SELECT count(*) FROM "' + name.replace('"', '""') + '"').fetchone()[0] for (name,) in tables}
        with closing(sqlite3.connect(temp / "users.db")) as db:
            account_count = db.execute("SELECT count(*) FROM users").fetchone()[0]
            algorithms = {}
            for (password_hash,) in db.execute("SELECT password_hash FROM users"):
                if not isinstance(password_hash, str) or "$" not in password_hash:
                    raise ValueError("invalid account password hash format")
                algorithm = password_hash.split("$", 1)[0]
                algorithms[algorithm] = algorithms.get(algorithm, 0) + 1
        warehouse = temp / "warehouse"
        warehouse.mkdir(mode=0o700)
        os.chmod(warehouse, 0o700)
        project_fd = os.open("projects", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=root_fd)
        try:
            (warehouse / "projects").mkdir(mode=0o700)
            os.chmod(warehouse / "projects", 0o700)
            files = []
            copy_tree(project_fd, warehouse / "projects", warehouse, files)
        finally:
            os.close(project_fd)
        try:
            os.stat("storage_config.json", dir_fd=root_fd, follow_symlinks=False)
        except FileNotFoundError:
            config_present = False
        else:
            entry = copy_file(root_fd, "storage_config.json", warehouse / "storage_config.json")
            entry["path"] = "storage_config.json"
            files.append(entry)
            config_present = True
        manifest = {"gpu_tables": counts, "warehouse_files": files,
                    "storage_config_present": config_present,
                    "accounts": {"count": account_count, "hash_algorithms": algorithms}}
        with open(temp / "manifest.json", "x", encoding="utf-8") as file:
            os.chmod(temp / "manifest.json", 0o600)
            json.dump(manifest, file, ensure_ascii=False, indent=2, sort_keys=True)
            file.write("\n")
        publish(temp, output)
        temp = None
    finally:
        os.close(root_fd)
        if temp is not None:
            shutil.rmtree(temp)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gpu-db", required=True)
    parser.add_argument("--warehouse-root", required=True)
    parser.add_argument("--auth-db", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        snapshot(args.gpu_db, args.warehouse_root, args.auth_db, args.output)
    except (OSError, ValueError, sqlite3.Error) as error:
        print(f"snapshot failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
