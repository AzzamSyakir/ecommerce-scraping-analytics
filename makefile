# docker command
start-docker:
	clear && docker compose  --env-file ./.env -f ./deployments/docker-compose.yml up -d 

stop-docker:
	clear && docker compose --env-file ./.env -f ./deployments/docker-compose.yml down --remove-orphans

clean-docker:
	clear && docker system prune && docker volume prune && docker image prune -a -f && docker container prune && docker buildx prune

start-db:
	clear && docker compose --env-file ./.env -f ./docker/docker-compose.yml up user-db product-db order-db auth-db -d