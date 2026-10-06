from fastapi import APIRouter
from fastapi.responses import JSONResponse
from app.config import settings
from app.db.mongo import mongo_manager

router = APIRouter(tags=["Health"])


@router.get(
    "/health",
    summary="Service Health Check",
    description="Returns microservice health and MongoDB connectivity status with graceful fallback.",
)
async def health_check():
    is_connected = await mongo_manager.ping()
    mongo_status = "connected" if is_connected else "fallback"

    return JSONResponse(
        status_code=200,
        content={
            "status": "ok",
            "service": settings.SERVICE_NAME,
            "database": "mongodb",
            "mongodb": mongo_status,
        },
    )
