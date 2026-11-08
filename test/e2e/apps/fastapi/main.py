"""FastAPI app for the versaDeploy e2e (behind uvicorn)."""
import os

from fastapi import FastAPI
from fastapi.responses import PlainTextResponse

app = FastAPI()


@app.get("/", response_class=PlainTextResponse)
def index():
    return f"versa-e2e fastapi {os.environ.get('APP_GREETING', '')}\n"
