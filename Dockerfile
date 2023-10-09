FROM golang:1.20.5-buster as go
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOBIN=/bin
RUN go install github.com/go-delve/delve/cmd/dlv@v1.8.2

FROM go as build
WORKDIR /build
COPY go.mod go.sum ./
COPY pkg ./pkg
RUN go build ./pkg/imports
COPY . .
RUN go build -o /bin/exclude-prefixes .

FROM build as test
CMD go test -test.v ./... -count=100

FROM test as debug
CMD dlv -l :40000 --headless=true --api-version=2 test -test.v ./...

FROM alpine as runtime
COPY --from=build /bin/exclude-prefixes /bin/exclude-prefixes
ENTRYPOINT ["/bin/exclude-prefixes"]