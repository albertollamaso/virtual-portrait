FROM golang

WORKDIR $GOPATH/src/github.com/virtual-portrait

COPY . .

RUN go get -d -v ./...

RUN go install -v ./...

CMD ["virtual-portrait"]