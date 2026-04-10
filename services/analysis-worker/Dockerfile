FROM python:3.14.3-slim-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libpq-dev \
    python3-dev \
    && rm -rf /var/lib/apt/lists/*

COPY services/analysis-worker/requirements.txt .

RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

# Download SentiWS lexicon (CC-BY-SA 4.0, Leipzig University)
RUN apt-get update && apt-get install -y --no-install-recommends unzip \
    && mkdir -p /sentiws \
    && python -c "import urllib.request; urllib.request.urlretrieve('https://downloads.wortschatz-leipzig.de/etc/SentiWS/SentiWS_v2.0.zip', '/tmp/sentiws.zip')" \
    && unzip /tmp/sentiws.zip -d /sentiws \
    && rm /tmp/sentiws.zip \
    && rm -rf /var/lib/apt/lists/*

FROM python:3.14.3-slim-bookworm

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --only-upgrade libssl3 openssl \
    && apt-get install -y --no-install-recommends libpq5 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /install /usr/local
COPY services/analysis-worker/ .
COPY --from=builder /sentiws /app/data/sentiws

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "main.py"]
