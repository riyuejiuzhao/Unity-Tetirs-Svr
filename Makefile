bin: go-proto
	go build .

docker-build: go-proto
	docker build -t tetris:latest .

go-proto:
	protoc --go_out=./proto/ \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_out=./proto/ \
		--proto_path=./proto \
		./proto/tetris.proto

run:
	docker run -p 50051 --name tetris-svr tetris:latest

start:
	docker start tetris-svr

clean:
	rm -rf proto/*.go