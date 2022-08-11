kind create cluster --config=config.yaml
kind load docker-image boletia/seventos 
kind load docker-image boletia/sreservas 
kind load docker-image boletia/ssevento 
kind load docker-image boletia/ssreserva 
kind load docker-image boletia/ssinventario 
kind load docker-image boletia/ssnotificacion
kubectl apply -f clientes.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx ^
  --for=condition=ready pod ^
  --selector=app.kubernetes.io/component=controller ^
  --timeout=180s
kubectl apply -f ingress.yaml
