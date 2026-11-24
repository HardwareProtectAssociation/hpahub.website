# -*- coding: utf-8 -*-
"""测试数据储存仓库 (Test Data Storage Warehouse)

A Flask web application for managing studio test data: projects, categorized
file uploads, and file management.

Run:
    pip install flask
    python app.py
"""

import os
import re
import json
import shutil
import logging
import subprocess
from datetime import datetime
from functools import wraps

from urllib.parse import quote, unquote

from flask import (
    Flask,
    jsonify,
    request,
    send_from_directory,
    abort,
)

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

DATA_ROOT = "/app/data/projects"
META_FILENAME = ".meta.json"

# File extension -> category mapping
IMAGE_EXTS = {"jpg", "jpeg", "png", "gif", "webp", "bmp", "svg"}
VIDEO_EXTS = {"mp4", "avi", "mkv", "mov", "wmv", "flv", "webm"}
DOC_EXTS = {
    "txt", "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "csv",
}

CATEGORY_IMAGES = "images"
CATEGORY_VIDEOS = "videos"
CATEGORY_DOCS = "docs"
CATEGORY_OTHER = "others"

MAX_CONTENT_LENGTH = 500 * 1024 * 1024

app = Flask(__name__)
app.config["MAX_CONTENT_LENGTH"] = MAX_CONTENT_LENGTH
app.config["JSON_AS_ASCII"] = False

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger("warehouse")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def ensure_data_root():
    """Create the data root directory if it does not exist."""
    os.makedirs(DATA_ROOT, exist_ok=True)


def json_error(message, status=400):
    """Return a JSON error response."""
    return jsonify({"ok": False, "error": message}), status


def safe_segment(name):
    """Validate a single path segment (project name / file name).

    Rejects empty names, path separators, traversal sequences and reserved
    Windows device names. Returns the sanitized name or raises ValueError.
    """
    if not isinstance(name, str):
        raise ValueError("名称必须是字符串")
    name = name.strip()
    if not name:
        raise ValueError("名称不能为空")
    if len(name) > 255:
        raise ValueError("名称过长（最多 255 个字符）")
    # Path separators and traversal
    if any(sep in name for sep in ("/", "\\", "\x00")):
        raise ValueError("名称不能包含路径分隔符")
    if name in (".", ".."):
        raise ValueError("名称非法")
    if re.search(r"[\x00-\x1f]", name):
        raise ValueError("名称包含非法控制字符")
    # Windows-invalid filename characters (also blocks ? and * glob injection)
    if re.search(r'[<>:"|?*]', name):
        raise ValueError('名称不能包含 < > : " | ? * 等非法字符')
    # Windows silently strips trailing dots/spaces -> reject to avoid surprises
    if name != name.rstrip(". "):
        raise ValueError("名称不能以点或空格结尾")
    # Reserved Windows device names
    stem = name.split(".")[0].upper()
    if stem in {"CON", "PRN", "AUX", "NUL"} or re.fullmatch(
        r"(COM|LPT)[1-9]", stem
    ):
        raise ValueError("名称为系统保留名，请更换")
    return name


def project_dir(project_name):
    """Resolve the on-disk directory for a project (must exist & be safe)."""
    safe_segment(project_name)
    path = os.path.join(DATA_ROOT, project_name)
    if os.path.islink(path):
        raise ValueError("非法路径")
    # Defence-in-depth: verify the resolved path stays inside DATA_ROOT.
    real_root = os.path.realpath(DATA_ROOT)
    real_path = os.path.realpath(path)
    if real_path != real_root and not real_path.startswith(real_root + os.sep):
        raise ValueError("非法路径")
    return path


def classify_file(filename):
    """Return the category key for a filename based on its extension."""
    ext = filename.rsplit(".", 1)[-1].lower() if "." in filename else ""
    if ext in IMAGE_EXTS:
        return CATEGORY_IMAGES
    if ext in VIDEO_EXTS:
        return CATEGORY_VIDEOS
    if ext in DOC_EXTS:
        return CATEGORY_DOCS
    return CATEGORY_OTHER


def unique_path(directory, filename):
    """Return a non-colliding path for *filename* inside *directory*.

    If the name already exists, appends _1, _2, ... before the extension.
    """
    name, ext = os.path.splitext(filename)
    candidate = filename
    counter = 1
    while os.path.exists(os.path.join(directory, candidate)):
        candidate = f"{name}_{counter}{ext}"
        counter += 1
    return os.path.join(directory, candidate), candidate


def media_url(kind, project, category, filename):
    segments = (kind, project, category, filename)
    return "/api/hpahub/warehouse/media/" + "/".join(quote(s, safe="") for s in segments)


def file_entry(category, filename, path):
    """Build the JSON metadata dict for a stored file."""
    stat = os.stat(path)
    entry = {
        "name": filename,
        "category": category,
        "size": stat.st_size,
        "modified": datetime.fromtimestamp(stat.st_mtime).isoformat(
            timespec="seconds"
        ),
        "url": media_url("file", os.path.basename(os.path.dirname(os.path.dirname(path))), category, filename),
    }
    # For videos: include thumbnail URL and codec info
    if category == CATEGORY_VIDEOS:
        cat_dir = os.path.dirname(path)
        thumb = _thumb_path(cat_dir, filename)
        if os.path.isfile(thumb):
            entry["thumb"] = media_url("thumb", os.path.basename(os.path.dirname(cat_dir)), category, filename)
        web = _web_path(cat_dir, filename)
        if os.path.isfile(web):
            entry["web_url"] = media_url("web", os.path.basename(os.path.dirname(cat_dir)), category, filename)
        prev = _preview_path(cat_dir, filename)
        if os.path.isfile(prev):
            entry["preview_url"] = media_url("preview", os.path.basename(os.path.dirname(cat_dir)), category, filename)
        entry["codec"] = video_codec(path)
    return entry


def list_project_files(project_name):
    """List all files in a project, grouped by category."""
    pdir = project_dir(project_name)
    if not os.path.isdir(pdir):
        return None
    result = {CATEGORY_IMAGES: [], CATEGORY_VIDEOS: [], CATEGORY_DOCS: [],
              CATEGORY_OTHER: []}
    for category in (CATEGORY_IMAGES, CATEGORY_VIDEOS, CATEGORY_DOCS,
                     CATEGORY_OTHER):
        cdir = os.path.join(pdir, category)
        if not os.path.isdir(cdir):
            continue
        for entry in sorted(os.listdir(cdir)):
            fpath = os.path.join(cdir, entry)
            if os.path.isfile(fpath) and not entry.startswith("."):
                result[category].append(file_entry(category, entry, fpath))
    return result


def project_meta_path(project_name):
    return os.path.join(project_dir(project_name), META_FILENAME)


def load_meta(project_name):
    """Load project metadata (display name, timestamps) if present."""
    try:
        with open(project_meta_path(project_name), "r", encoding="utf-8") as fh:
            return json.load(fh)
    except (OSError, ValueError):
        return {}


def save_meta(project_name, meta):
    with open(project_meta_path(project_name), "w", encoding="utf-8") as fh:
        json.dump(meta, fh, ensure_ascii=False, indent=2)


def requester_identity():
    """Return the identity supplied by the trusted proxy."""
    uid = (request.headers.get("X-User-Id") or "").strip()
    uname = unquote((request.headers.get("X-User-Name") or "").strip())
    if uid or uname:
        return {"id": uid or uname, "name": uname or uid}
    return None


def dir_size(path):
    """Return total byte size of all files under *path* (recursive)."""
    total = 0
    try:
        for dirpath, _dirnames, filenames in os.walk(path):
            for fn in filenames:
                fp = os.path.join(dirpath, fn)
                try:
                    total += os.path.getsize(fp)
                except OSError:
                    pass
    except OSError:
        pass
    return total


def disk_usage():
    """Return total / used / free bytes for the data partition."""
    try:
        usage = shutil.disk_usage(DATA_ROOT)
        return {"total": usage.total, "used": usage.used, "free": usage.free}
    except OSError:
        return {"total": 0, "used": 0, "free": 0}


def list_disks():
    """List all mounted disks with usage info."""
    disks = []
    seen = set()
    try:
        with open("/proc/mounts") as f:
            for line in f:
                parts = line.split()
                if len(parts) < 3:
                    continue
                device, mountpoint, fstype = parts[0], parts[1], parts[2]
                if not device.startswith("/dev/") or mountpoint.startswith(("/proc", "/sys", "/dev", "/run", "/snap", "/boot")):
                    continue
                if fstype not in ("ext4", "ext3", "ext2", "xfs", "btrfs", "vfat", "ntfs", "fuseblk"):
                    continue
                key = mountpoint
                if key in seen:
                    continue
                seen.add(key)
                try:
                    usage = shutil.disk_usage(mountpoint)
                    disks.append({
                        "device": device,
                        "mountpoint": mountpoint,
                        "fstype": fstype,
                        "total": usage.total,
                        "used": usage.used,
                        "free": usage.free,
                    })
                except OSError:
                    pass
    except OSError:
        pass
    # Always include the data root's disk even if not found above
    if not any(d["mountpoint"] == "/" for d in disks):
        try:
            usage = shutil.disk_usage("/")
            disks.insert(0, {
                "device": "root",
                "mountpoint": "/",
                "fstype": "unknown",
                "total": usage.total,
                "used": usage.used,
                "free": usage.free,
            })
        except OSError:
            pass
    return disks


def category_storage_usage():
    """Calculate total bytes used per file category across all projects."""
    usage = {
        CATEGORY_IMAGES: 0,
        CATEGORY_VIDEOS: 0,
        CATEGORY_DOCS: 0,
        CATEGORY_OTHER: 0,
    }
    ensure_data_root()
    for proj in os.listdir(DATA_ROOT):
        pdir = os.path.join(DATA_ROOT, proj)
        if not os.path.isdir(pdir) or proj.startswith("."):
            continue
        for cat in usage:
            cdir = os.path.join(pdir, cat)
            if not os.path.isdir(cdir):
                continue
            for entry in os.listdir(cdir):
                fpath = os.path.join(cdir, entry)
                if os.path.isfile(fpath):
                    try:
                        usage[cat] += os.path.getsize(fpath)
                    except OSError:
                        pass
    return usage


STORAGE_CONFIG_FILE = os.path.join(os.path.dirname(DATA_ROOT), "storage_config.json")


def load_storage_config():
    """Load storage allocation config."""
    try:
        with open(STORAGE_CONFIG_FILE, "r") as f:
            return json.load(f)
    except (OSError, ValueError):
        return {"allocations": {}}


def save_storage_config(config):
    with open(STORAGE_CONFIG_FILE, "w") as f:
        json.dump(config, f, ensure_ascii=False, indent=2)


# ---------------------------------------------------------------------------
# Video thumbnail / web preview via ffmpeg
# ---------------------------------------------------------------------------

FFMPEG = "ffmpeg"
FFPROBE = "ffprobe"
THUMB_DIR = ".thumbs"          # per-category subdirectory for thumbs
WEB_PREVIEW_DIR = ".web"       # per-category subdirectory for web previews


def _thumb_path(category_dir, filename):
    return os.path.join(category_dir, THUMB_DIR, filename + ".jpg")


def _web_path(category_dir, filename):
    return os.path.join(category_dir, WEB_PREVIEW_DIR, filename)


def video_codec(filepath):
    """Return the video codec name (e.g. 'hevc', 'h264') or '' on error."""
    try:
        out = subprocess.check_output(
            [FFPROBE, "-v", "quiet", "-select_streams", "v:0",
             "-show_entries", "stream=codec_name", "-of", "csv=p=0", filepath],
            timeout=15,
        )
        return out.decode().strip()
    except Exception:
        return ""


def generate_thumbnail(category_dir, filename):
    """Extract first-frame JPG thumbnail for a video file. Returns thumb path or None."""
    src = os.path.join(category_dir, filename)
    thumb = _thumb_path(category_dir, filename)
    if os.path.isfile(thumb):
        return thumb
    os.makedirs(os.path.dirname(thumb), exist_ok=True)
    try:
        subprocess.run(
            [FFMPEG, "-y", "-ss", "1", "-i", src,
             "-vframes", "1", "-vf", "scale=480:-2", "-q:v", "4", thumb],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30,
        )
        return thumb if os.path.isfile(thumb) else None
    except Exception:
        return None


def generate_web_preview(category_dir, filename):
    """Transcode video to H.264 web-friendly MP4. Returns path or None."""
    src = os.path.join(category_dir, filename)
    dst = _web_path(category_dir, filename)
    if os.path.isfile(dst):
        return dst
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    try:
        subprocess.run(
            [FFMPEG, "-y", "-i", src,
             "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
             "-vf", "scale='min(1280,iw)':-2",
             "-c:a", "aac", "-b:a", "128k",
             "-movflags", "+faststart", dst],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            timeout=600,
        )
        return dst if os.path.isfile(dst) else None
    except Exception:
        return None


PREVIEW_DIR = ".preview"  # super low-res previews for inline card playback


def _preview_path(category_dir, filename):
    return os.path.join(category_dir, PREVIEW_DIR, filename)


def generate_lowres_preview(category_dir, filename):
    """Generate a super low-quality preview (240p, no audio) for inline card playback."""
    src = os.path.join(category_dir, filename)
    dst = _preview_path(category_dir, filename)
    if os.path.isfile(dst):
        return dst
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    try:
        subprocess.run(
            [FFMPEG, "-y", "-i", src,
             "-vf", "scale=240:-2",
             "-c:v", "libx264", "-preset", "ultrafast", "-crf", "36",
             "-an",  # no audio — saves bandwidth
             "-movflags", "+faststart", dst],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
            timeout=300,
        )
        return dst if os.path.isfile(dst) else None
    except Exception:
        return None


def _move_video_sidecars(category_dir, old_name, new_name):
    """改名时把缩略图 / 网页预览 / 低清预览一起搬走，否则新名字下找不到预览文件。"""
    for path_of in (_thumb_path, _web_path, _preview_path):
        old = path_of(category_dir, old_name)
        new = path_of(category_dir, new_name)
        if os.path.isfile(old):
            os.makedirs(os.path.dirname(new), exist_ok=True)
            os.replace(old, new)


def project_summary(project_name):
    """Build the JSON summary dict for one project."""
    pdir = project_dir(project_name)
    meta = load_meta(project_name)
    counts = {
        CATEGORY_IMAGES: 0,
        CATEGORY_VIDEOS: 0,
        CATEGORY_DOCS: 0,
        CATEGORY_OTHER: 0,
    }
    total = 0
    size_bytes = 0
    if os.path.isdir(pdir):
        for category in counts:
            cdir = os.path.join(pdir, category)
            if not os.path.isdir(cdir):
                continue
            for entry in os.listdir(cdir):
                fpath = os.path.join(cdir, entry)
                if os.path.isfile(fpath) and not entry.startswith("."):
                    counts[category] += 1
                    total += 1
                    try:
                        size_bytes += os.path.getsize(fpath)
                    except OSError:
                        pass
    created = meta.get("created")
    if not created:
        try:
            created = datetime.fromtimestamp(os.path.getctime(pdir)).isoformat(
                timespec="seconds"
            )
        except OSError:
            created = None
    return {
        "name": project_name,
        "display_name": meta.get("display_name", project_name),
        "created": created,
        "counts": counts,
        "total_files": total,
        "size_bytes": size_bytes,
        "creator_id": meta.get("creator_id"),
        "creator_name": meta.get("creator_name"),
    }


def handle_api_errors(fn):
    """Decorator turning ValueError / OSError into clean JSON errors."""
    @wraps(fn)
    def wrapper(*args, **kwargs):
        try:
            return fn(*args, **kwargs)
        except ValueError as exc:
            return json_error(str(exc), 400)
        except OSError as exc:
            log.exception("OS error in %s", fn.__name__)
            return json_error("服务器文件操作失败: {}".format(exc), 500)
        except Exception:  # noqa: BLE001 - last-resort guard for API layer
            log.exception("Unhandled error in %s", fn.__name__)
            return json_error("服务器内部错误", 500)
    return wrapper



# ---------------------------------------------------------------------------
# Project API
# ---------------------------------------------------------------------------

@app.route("/api/projects", methods=["GET"])
@handle_api_errors
def api_list_projects():
    ensure_data_root()
    projects = []
    for entry in sorted(os.listdir(DATA_ROOT)):
        pdir = os.path.join(DATA_ROOT, entry)
        if os.path.isdir(pdir) and not entry.startswith("."):
            projects.append(project_summary(entry))
    return jsonify({"ok": True, "projects": projects, "disk": disk_usage(),
                    "me": requester_identity()})


@app.route("/api/storage", methods=["GET"])
@handle_api_errors
def api_get_storage():
    """Get full storage info: disks, category usage, allocations."""
    disks = list_disks()
    categories = category_storage_usage()
    config = load_storage_config()
    return jsonify({
        "ok": True,
        "disks": disks,
        "categories": categories,
        "allocations": config.get("allocations", {}),
    })


@app.route("/api/storage", methods=["PUT", "PATCH"])
@handle_api_errors
def api_set_storage():
    """Update storage allocation config."""
    data = request.get_json(silent=True) or {}
    config = load_storage_config()
    if "allocations" in data:
        config["allocations"] = data["allocations"]
    save_storage_config(config)
    return jsonify({"ok": True, "allocations": config.get("allocations", {})})


@app.route("/api/projects", methods=["POST"])
@handle_api_errors
def api_create_project():
    data = request.get_json(silent=True) or {}
    raw_name = data.get("name", "")
    # Project directory name: keep the user's display name but sanitize it.
    name = safe_segment(raw_name)
    if name.startswith("."):
        raise ValueError("名称非法")
    pdir = project_dir(name)
    if os.path.exists(pdir):
        return json_error("同名项目已存在", 409)
    os.makedirs(pdir)
    for category in (CATEGORY_IMAGES, CATEGORY_VIDEOS, CATEGORY_DOCS,
                     CATEGORY_OTHER):
        os.makedirs(os.path.join(pdir, category), exist_ok=True)
    meta = {
        "display_name": name,
        "created": datetime.now().isoformat(timespec="seconds"),
    }
    creator = requester_identity()
    if creator:
        meta["creator_id"] = creator["id"]
        meta["creator_name"] = creator["name"]
    save_meta(name, meta)
    log.info("Created project %r (creator=%r)", name, meta.get("creator_id"))
    return jsonify({"ok": True, "project": project_summary(name)}), 201


@app.route("/api/projects/<project_name>", methods=["GET"])
@handle_api_errors
def api_get_project(project_name):
    files = list_project_files(project_name)
    if files is None:
        return json_error("项目不存在", 404)
    return jsonify({
        "ok": True,
        "project": project_summary(project_name),
        "files": files,
    })


@app.route("/api/projects/<project_name>", methods=["PUT", "PATCH"])
@handle_api_errors
def api_rename_project(project_name):
    old_dir = project_dir(project_name)
    if not os.path.isdir(old_dir):
        return json_error("项目不存在", 404)
    data = request.get_json(silent=True) or {}
    new_name = safe_segment(data.get("name", ""))
    if new_name.startswith("."):
        raise ValueError("名称非法")
    if new_name == project_name:
        return jsonify({"ok": True, "project": project_summary(project_name)})
    new_dir = project_dir(new_name)
    if os.path.exists(new_dir):
        return json_error("同名项目已存在", 409)
    os.rename(old_dir, new_dir)
    meta = load_meta(new_name)
    meta["display_name"] = new_name
    meta["renamed"] = datetime.now().isoformat(timespec="seconds")
    save_meta(new_name, meta)
    log.info("Renamed project %r -> %r", project_name, new_name)
    return jsonify({"ok": True, "project": project_summary(new_name)})


@app.route("/api/projects/<project_name>", methods=["DELETE"])
@handle_api_errors
def api_delete_project(project_name):
    pdir = project_dir(project_name)
    if not os.path.isdir(pdir):
        return json_error("项目不存在", 404)
    shutil.rmtree(pdir)
    log.info("Deleted project %r", project_name)
    return jsonify({"ok": True})


# ---------------------------------------------------------------------------
# File API
# ---------------------------------------------------------------------------

@app.route("/api/projects/<project_name>/files", methods=["POST"])
@handle_api_errors
def api_upload_files(project_name):
    pdir = project_dir(project_name)
    if not os.path.isdir(pdir):
        return json_error("项目不存在", 404)

    uploads = request.files.getlist("files")
    # Optional custom names, same order as the `files` field.
    custom_names = request.form.getlist("names")
    if not uploads or all(not f.filename for f in uploads):
        return json_error("没有收到文件", 400)

    saved = []
    for index, storage in enumerate(uploads):
        original = storage.filename or ""
        # The browser sends a full path in some cases; keep only the basename.
        original = original.replace("\\", "/").rsplit("/", 1)[-1]
        if not original:
            continue
        try:
            safe_segment(original)
        except ValueError:
            # Fall back to a safe generated name for hostile filenames.
            ext = ""
            if "." in original:
                ext = "." + original.rsplit(".", 1)[-1][:16]
            original = "file_{}{}".format(index, ext)
            safe_segment(original)

        # Use the user-edited name when provided.
        if index < len(custom_names) and custom_names[index].strip():
            try:
                original = safe_segment(custom_names[index])
            except ValueError as exc:
                return json_error(
                    "文件名「{}」无效: {}".format(custom_names[index], exc), 400
                )

        if original.startswith("."):
            raise ValueError("名称非法")
        category = classify_file(original)
        cdir = os.path.join(pdir, category)
        if os.path.islink(cdir):
            raise ValueError("非法路径")
        os.makedirs(cdir, exist_ok=True)
        final_path, final_name = unique_path(cdir, original)
        storage.save(final_path)
        saved.append(file_entry(category, final_name, final_path))
        # Generate thumbnail and low-res preview for videos
        if category == CATEGORY_VIDEOS:
            cat_dir = os.path.join(pdir, category)
            generate_thumbnail(cat_dir, final_name)
            generate_lowres_preview(cat_dir, final_name)

    if not saved:
        return json_error("没有收到有效文件", 400)
    log.info("Uploaded %d file(s) to project %r", len(saved), project_name)
    return jsonify({"ok": True, "files": saved}), 201


@app.route(
    "/api/projects/<project_name>/files/<category>/<path:filename>",
    methods=["PUT", "PATCH"],
)
@handle_api_errors
def api_rename_file(project_name, category, filename):
    data = request.get_json(silent=True) or {}
    new_raw = data.get("name", "")
    new_name = safe_segment(new_raw)
    if not new_name:
        raise ValueError("名称不能为空")
    pdir = project_dir(project_name)
    old_path = checked_file(project_name, category, filename)
    new_path = os.path.join(os.path.dirname(old_path), new_name)
    if new_name.startswith("."):
        raise ValueError("名称非法")
    if not os.path.isfile(old_path):
        raise ValueError("文件不存在")
    if os.path.abspath(old_path) == os.path.abspath(new_path):
        return jsonify({"ok": True, "file": file_entry(category, new_name, new_path)})
    if os.path.exists(new_path):
        raise ValueError("同名文件已存在")
    os.rename(old_path, new_path)
    if category == CATEGORY_VIDEOS:
        _move_video_sidecars(os.path.dirname(old_path), filename, new_name)
    return jsonify({"ok": True, "file": file_entry(category, new_name, new_path)})


@app.route(
    "/api/projects/<project_name>/files/<category>/<path:filename>",
    methods=["DELETE"],
)
@handle_api_errors
def api_delete_file(project_name, category, filename):
    fpath = checked_file(project_name, category, filename)
    if not os.path.isfile(fpath):
        return json_error("文件不存在", 404)
    os.remove(fpath)
    log.info("Deleted file %r (%s) from %r", filename, category, project_name)
    return jsonify({"ok": True})



@app.route("/api/projects/<project_name>/files/<category>/<path:filename>/transcode", methods=["POST"])
@handle_api_errors
def api_transcode_video(project_name, category, filename):
    """Trigger web-preview transcode for a video."""
    fpath = checked_file(project_name, category, filename)
    if not os.path.isfile(fpath):
        raise ValueError("文件不存在")
    cat_dir = os.path.dirname(fpath)
    result = generate_web_preview(cat_dir, filename)
    if result:
        return jsonify({"ok": True, "web_url": media_url("web", project_name, category, filename)})
    raise ValueError("转码失败")


def checked_file(project, category, filename, kind="file"):
    safe_segment(project)
    safe_segment(filename)
    if project.startswith(".") or filename.startswith("."):
        raise ValueError("名称非法")
    if category not in (CATEGORY_IMAGES, CATEGORY_VIDEOS, CATEGORY_DOCS, CATEGORY_OTHER):
        raise ValueError("非法文件分类")
    if kind not in ("file", "thumb", "web", "preview") or (kind != "file" and category != CATEGORY_VIDEOS):
        raise ValueError("非法媒体类型")
    pdir = project_dir(project)
    cdir = os.path.join(pdir, category)
    root = os.path.realpath(DATA_ROOT)
    real_project = os.path.realpath(pdir)
    real_category = os.path.realpath(cdir)
    if not real_project.startswith(root + os.sep) or not real_category.startswith(real_project + os.sep):
        raise ValueError("非法路径")
    paths = {"file": lambda d, f: os.path.join(d, f), "thumb": _thumb_path,
             "web": _web_path, "preview": _preview_path}
    result = paths[kind](cdir, filename)
    if (os.path.islink(pdir) or os.path.islink(cdir) or
            os.path.islink(os.path.dirname(result)) or os.path.islink(result)):
        raise ValueError("非法路径")
    if not os.path.realpath(result).startswith(real_category + os.sep):
        raise ValueError("非法路径")
    return result


@app.route("/api/hpahub/media/<kind>/<project>/<category>/<filename>", methods=["GET", "HEAD"])
def serve_media(kind, project, category, filename):
    try:
        path = checked_file(project, category, filename, kind)
    except ValueError:
        abort(404)
    if not os.path.isfile(path):
        abort(404)
    response = send_from_directory(os.path.dirname(path), os.path.basename(path), conditional=True)
    response.headers["Cache-Control"] = "private, no-store"
    response.headers["X-Content-Type-Options"] = "nosniff"
    if response.mimetype not in {"image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "video/webm"}:
        response.headers["Content-Disposition"] = "attachment"
    return response


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    ensure_data_root()
    log.info("Data directory: %s", DATA_ROOT)
    app.run(host="0.0.0.0", port=18080, debug=False, threaded=True)
