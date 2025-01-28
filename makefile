# docker command
start-docker:
	clear && docker compose  --env-file ./.env -f ./deployments/docker-compose.yml up -d 

stop-docker:
	clear && \
	docker compose --env-file ./.env -f ./deployments/docker-compose.yml down --remove-orphans

clean-docker:
	clear && \
	docker compose --env-file ./.env -f ./deployments/docker-compose.yml down --volumes --rmi all && \
	docker builder prune -f