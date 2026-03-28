import os
from minio import Minio
import clickhouse_connect

def init_minio() -> Minio:
    """
    Initializes and returns the MinIO client using environment variables.
    """
    return Minio(
        os.getenv("MINIO_ENDPOINT", "localhost:9000"),
        access_key=os.getenv("MINIO_ROOT_USER", "aer_admin"),
        secret_key=os.getenv("MINIO_ROOT_PASSWORD", "aer_password_123"),
        secure=False
    )

def init_clickhouse():
    """
    Initializes the ClickHouse client and ensures the Gold database 
    and required tables are provisioned.
    """
    client = clickhouse_connect.get_client(
        host='localhost', 
        port=8123, 
        username=os.getenv("CLICKHOUSE_USER", "aer_admin"), 
        password=os.getenv("CLICKHOUSE_PASSWORD", "aer_password_123")
    )

    # Automatically provision the Gold database and table if they do not exist
    client.command('CREATE DATABASE IF NOT EXISTS aer_gold')
    client.command(
        'CREATE TABLE IF NOT EXISTS aer_gold.metrics '
        '(timestamp DateTime, value Float64) '
        'ENGINE = MergeTree() ORDER BY timestamp'
    )
    
    return client