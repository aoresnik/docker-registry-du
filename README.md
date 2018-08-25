# docker-registry-du
Report storage usage of a Docker Registry. Reports usage by image and tag, shared and exclusive usage.

## Installing

1. You need a working Go installation
1. Install dedpendencies
```
go get github.com/heroku/docker-registry-client/registry
go get golang.org/x/crypto/ssh/terminal
```
3. Install docker-registry-du:
```
go get github.com/aoresnik/docker-registry-du
```
