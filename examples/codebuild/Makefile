build:
	docker build -t mumoshu/variant-runner-codebuild:canary .

push:
	docker push mumoshu/variant-runner-codebuild:canary

run:
	docker run --env-file .env --rm -it -v $$HOME/.aws/credentials:/root/.aws/credentials mumoshu/variant-runner-codebuild:canary 'echo yes'
