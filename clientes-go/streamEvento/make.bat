set GOOS=linux
go build .
docker image build -t boletia/ssevento .