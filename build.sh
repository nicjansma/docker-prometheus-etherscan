docker build -t prometheus-etherscan -f Dockerfile .
docker tag prometheus-etherscan:latest nicjansma/prometheus-etherscan:latest
docker push nicjansma/prometheus-etherscan:latest
