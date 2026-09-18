FROM golang:1.24 AS build
WORKDIR /src
COPY reconix/go.mod reconix/go.sum /src/reconix/
RUN cd /src/reconix && go mod download
COPY reconix /src/reconix
RUN cd /src/reconix && CGO_ENABLED=0 go build -o /out/reconix ./cmd/reconix
COPY reconix-cloud/go.mod reconix-cloud/go.sum /src/reconix-cloud/
RUN cd /src/reconix-cloud && go mod download
COPY reconix-cloud /src/reconix-cloud
RUN cd /src/reconix-cloud && CGO_ENABLED=0 go build -o /out/reconix-cloud-api ./cmd/api
RUN cd /src/reconix-cloud && CGO_ENABLED=0 go build -o /out/reconix-cloud-worker ./cmd/worker

FROM alpine:3.21
RUN addgroup -S reconix && adduser -S -G reconix reconix
WORKDIR /opt/reconix
COPY --from=build /out/reconix /usr/local/bin/reconix
COPY --from=build /out/reconix-cloud-api /usr/local/bin/reconix-cloud-api
COPY --from=build /out/reconix-cloud-worker /usr/local/bin/reconix-cloud-worker
COPY --from=build /src/reconix/config ./config
COPY --from=build /src/reconix/templates ./templates
USER reconix
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/reconix-cloud-api"]
