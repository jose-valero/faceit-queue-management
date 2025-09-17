ARCH=amd64
ECR_REPO=faceit-bot
REGION=us-east-1
ACCOUNT_ID?=506636091874
IMAGE_TAG?=v0.1.0

bot-image:
	docker build -t $(ECR_REPO):$(IMAGE_TAG) .

ecr-login:
	aws ecr get-login-password --region $(REGION) \
	| docker login --username AWS --password-stdin $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com

ecr-create:
	aws ecr create-repository --repository-name $(ECR_REPO) --region $(REGION) || true

bot-push: ecr-login ecr-create
	docker tag $(ECR_REPO):$(IMAGE_TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):$(IMAGE_TAG)
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(ECR_REPO):$(IMAGE_TAG)

build-webhook:
	rm -rf dist/webhook && mkdir -p dist/webhook
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -C ./cmd/webhook -o ../../dist/webhook/bootstrap .
	cd dist/webhook && zip -r function.zip bootstrap

build-janitor:
	rm -rf dist/janitor && mkdir -p dist/janitor
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -C ./cmd/janitor -o ../../dist/janitor/bootstrap .
	cd dist/janitor && zip -r function.zip bootstrap
