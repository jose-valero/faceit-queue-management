ARCH=amd64
ECR_REPO=dev-faceit-cluster-app
ECR_DB_REPO=dev-faceit-cluster-postgres
REGION=us-east-1
ACCOUNT_ID?=506636091874
IMAGE_TAG?=$(shell git rev-parse --short HEAD)
POSTGRES_VERSION?=17.6-alpine

bot-build:
	./build.sh

bot-image: bot-build
	$(eval IMAGE_TAG=$(shell git rev-parse --short HEAD))
	AWS_PROFILE=cpx-valero docker build -t $(ECR_REPO):$(IMAGE_TAG) -t $(ECR_REPO):latest .
ecr-login:
	AWS_PROFILE=cpx-valero  aws ecr get-login-password --region $(REGION) \
	| docker login --username AWS --password-stdin $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com

db-image:
	AWS_PROFILE=cpx-valero  docker build --build-arg POSTGRES_VERSION=$(POSTGRES_VERSION) -f Dockerfile-postgres -t $(ECR_DB_REPO):$(POSTGRES_VERSION) .

bot-push: ecr-login
	AWS_PROFILE=cpx-valero  docker tag $(ECR_REPO):$(IMAGE_TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):$(IMAGE_TAG)
	AWS_PROFILE=cpx-valero  docker tag $(ECR_REPO):latest $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):latest
	AWS_PROFILE=cpx-valero  docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):$(IMAGE_TAG)
	AWS_PROFILE=cpx-valero  docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):latest

db-push: ecr-login
	AWS_PROFILE=cpx-valero  docker tag $(ECR_DB_REPO):$(POSTGRES_VERSION) $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_DB_REPO):$(POSTGRES_VERSION)
	AWS_PROFILE=cpx-valero  docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_DB_REPO):$(POSTGRES_VERSION)

build-webhook:
	rm -rf dist/webhook && mkdir -p dist/webhook
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -C ./cmd/webhook -o ../../dist/webhook/bootstrap .
	cd dist/webhook && zip -r function.zip bootstrap

build-janitor:
	rm -rf dist/janitor && mkdir -p dist/janitor
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -C ./cmd/janitor -o ../../dist/janitor/bootstrap .
	cd dist/janitor && zip -r function.zip bootstrap

bot-deploy: bot-image bot-push
	AWS_PROFILE=cpx-valero go run update-ecs.go
