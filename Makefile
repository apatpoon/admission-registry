include .env

build-push:
     docker build -f Dockerfile -t pooncheebean/admission-registry:latest .
     docker push pooncheebean/admission-registry:latest
