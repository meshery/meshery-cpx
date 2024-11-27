protoc-setup:
	cd meshes
	wget https://raw.githubusercontent.com/layer5io/meshery/master/meshes/meshops.proto

proto:	
	protoc -I meshes/ meshes/meshops.proto --go_out=plugins=grpc:./meshes/

docker:
	DOCKER_BUILDKIT=1 docker build -t layer5/meshery-cpx .

docker-run:
	(docker rm -f meshery-cpx) || true
	docker run --name meshery-cpx -d \
	-p 10008:10008 \
	-e DEBUG=true \
	layer5/meshery-cpx

run:
	go$(v) mod tidy; \
	DEBUG=true GOPROXY=direct GOSUMDB=off go run main.go

error:
	go run github.com/layer5io/meshkit/cmd/errorutil -d . analyze -i ./helpers -o ./helpers