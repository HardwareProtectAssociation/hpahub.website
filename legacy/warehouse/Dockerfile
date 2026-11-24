FROM python:3.12-slim
ARG DEBIAN_MIRROR
ARG PIP_INDEX_URL=https://pypi.org/simple
WORKDIR /app
RUN if [ -n "$DEBIAN_MIRROR" ]; then sed -i "s|http://deb.debian.org|$DEBIAN_MIRROR|g" /etc/apt/sources.list.d/debian.sources; fi \
    && apt-get update && apt-get install -y --no-install-recommends ffmpeg && rm -rf /var/lib/apt/lists/*
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY app.py ./
CMD ["gunicorn", "--workers", "1", "--threads", "4", "--bind", "0.0.0.0:18080", "app:app"]
