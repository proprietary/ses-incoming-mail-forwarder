.PHONY = test
test: lambda.go lambda_test.go
	go test

bootstrap: lambda.go
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap lambda.go
	zip lambda.zip bootstrap

.PHONY = clean
clean:
	@rm lambda.zip bootstrap
