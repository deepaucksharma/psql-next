FROM python:3.9-slim

WORKDIR /tests

# Install test dependencies
RUN pip install pytest requests prometheus-client

# Copy test files
COPY . /tests/

CMD ["pytest", "-v"]