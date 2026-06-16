FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY locustfile.py start-locust.sh stop-locust.sh ./
RUN chmod +x start-locust.sh stop-locust.sh
ENV LOCUST_HOST=http://app-sidecar:8081
ENV REQUESTS_PER_SEC=20
EXPOSE 8089
CMD ["locust", "-f", "/app/locustfile.py", "--host", "http://app-sidecar:8081", "--web-host", "0.0.0.0", "--web-port", "8089"]
