docker stop land-nginx
docker stop land-fastdfs
docker rm land-nginx
docker rm land-fastdfs
docker-compose up -d
