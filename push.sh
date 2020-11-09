#eval $(minikube docker-env)
make docker-otelcol

docker tag otelcol:latest rakyll/otelcol
docker push rakyll/otelcol:latest

# docker tag otelcol:latest 516699956539.dkr.ecr.us-east-1.amazonaws.com/otelcol:latest
# docker push 516699956539.dkr.ecr.us-east-1.amazonaws.com/otelcol:latest

kubectl delete deployment otel-collector
kubectl apply -f docker-config.yaml
kubectl get pods