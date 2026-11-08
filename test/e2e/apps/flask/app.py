"""Flask app for the versaDeploy e2e (behind gunicorn)."""
import os

from flask import Flask

app = Flask(__name__)


@app.get("/")
def index():
    return f"versa-e2e flask {os.environ.get('APP_GREETING', '')}\n"
