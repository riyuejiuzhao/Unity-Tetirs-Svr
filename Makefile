bin: go-proto
	go build cmd/main.go

go-proto:
	protoc --go_out=./proto/ \
		--go_opt=paths=source_relative \
		--proto_path=./proto \
		./proto/*.proto

clean:
	rm -rf proto/*.go
	rm main