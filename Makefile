ARCH=amd64
ECR_REPO=faceit-bot
ECR_DB_REPO=postgres
REGION=us-east-1
ACCOUNT_ID?=506636091874
IMAGE_TAG?=v0.1.0
POSTGRES_VERSION?=18.0

bot-build:
	./build.sh

bot-image: bot-build
	AWS_PROFILE=cpx-valero docker build -t $(ECR_REPO):$(IMAGE_TAG) .
ecr-login:
	AWS_PROFILE=cpx-valero  aws ecr get-login-password --region $(REGION) \
	| docker login --username AWS --password-stdin $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com

ecr-create:
	aws ecr create-repository --repository-name $(ECR_REPO) --region $(REGION) || true

ecr-db-create:
	aws ecr create-repository --repository-name $(ECR_DB_REPO) --region $(REGION) || true

db-image:
	AWS_PROFILE=cpx-valero  docker build --build-arg POSTGRES_VERSION=$(POSTGRES_VERSION) -f Dockerfile.postgres -t $(ECR_DB_REPO):$(POSTGRES_VERSION) .

bot-push: ecr-login ecr-create
	AWS_PROFILE=cpx-valero  docker tag $(ECR_REPO):$(IMAGE_TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):$(IMAGE_TAG)
	AWS_PROFILE=cpx-valero  ocker push $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):$(IMAGE_TAG)

db-push: ecr-login ecr-db-create
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
