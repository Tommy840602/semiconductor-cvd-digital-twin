FROM python:3.12-slim
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 OPENBLAS_NUM_THREADS=1 OPENCV_IO_MAX_IMAGE_PIXELS=1048576
WORKDIR /app
COPY vision/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY vision/*.py ./
USER 1000:1000
CMD ["python", "worker.py"]
