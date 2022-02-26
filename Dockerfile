FROM golang:1.17-alpine
 
RUN mkdir /project
 
COPY . /project
 
WORKDIR /project
 
RUN go build -o main . 
 
ENTRYPOINT ["/project/main"]
