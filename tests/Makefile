.EXPORT_ALL_VARIABLES:

VAULT_ADDR=http://localhost:8200
VAULT_TOKEN=vaultsecure_acc_tests

tests:
	docker-compose up -d
	TF_ACC=1 go test -v ../vaultsecure/...
	docker-compose down

tests-stop:
	docker-compose down