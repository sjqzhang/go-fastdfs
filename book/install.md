# 极速体验{#quick}

### linux

```shell
wget --no-check-certificate  https://github.com/sjqzhang/go-fastdfs/releases/download/v1.3.1/fileserver -O fileserver && chmod +x fileserver && ./fileserver
```

### window
(注意：下载时要注意链接的版本号，windows下直接运行fileserver.exe，执行文件在这里[下载](https://github.com/sjqzhang/fastdfs/releases))



## 编译安装{#compiler_install}

### 如何编译(go1.9.2+)？

```
git clone https://github.com/sjqzhang/go-fastdfs.git
cd go-fastdfs
mv vendor src
pwd=`pwd`
GOPATH=$pwd go build -o fileserver fileserver.go
```

### 如何运行
```
./fileserver
```


# 小集群安装{#small-cluster}

```nginx
worker_processes  1;
events {
        worker_connections  1024;
}
http {
        include       mime.types;
        default_type  application/html;
        log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"';
        access_log  /var/log/nginx/access.log  main;
        error_log  /var/log/nginx/error.log  error;
        sendfile        on;
        keepalive_timeout  65;
		client_max_body_size 0; 
		proxy_redirect ~/big/upload/(.*) /big/upload/$1;  #继点续传一定要设置(注意)
        upstream go-fastdfs {
                server 10.1.14.36:8080;
                server 10.1.14.37:8080;
                ip_hash;     #notice:very important(注意)
        }
        server {
                listen       80;
                server_name  localhost;
                location / {
                    proxy_set_header Host $host; #notice:very important(注意)
                    proxy_set_header X-Real-IP $remote_addr; #notice:very important(注意)
                    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; #notice:very important(注意)
                    proxy_pass http://go-fastdfs;
                }

        }
}
```

## 海量集群安装{#big-cluster}

```nginx
worker_processes  1;
events {
        worker_connections  1024;
}
http {
        include       mime.types;
        default_type  application/html;
        log_format  main  '$remote_addr - $remote_user [$time_local] "$request" '
                      '$status $body_bytes_sent "$http_referer" '
                      '"$http_user_agent" "$http_x_forwarded_for"';
        access_log  /var/log/nginx/access.log  main;
        error_log  /var/log/nginx/error.log  error;
        sendfile        on;
        keepalive_timeout  65;
        rewrite_log on;
        client_max_body_size 0;
        proxy_redirect ~/(\w+)/big/upload/(.*) /$1/big/upload/$2;  #继点续传一定要设置(注意)
        #以下是一下横向扩展的配置，当前统一大集群容量不够时，只要增加一个小集群，也就是增加一个
        #upstream ,一个小群集内按业务需求设定副本数，也就是机器数。
        upstream gofastdfs-group1 {
                server 10.1.51.70:8082;
                #server 10.1.14.37:8082;
                ip_hash;     #notice:very important(注意)
        }
	upstream gofastdfs-group2 {
		server 10.1.51.70:8083;
                #server 10.1.14.36:8083;
                ip_hash;     #notice:very important(注意)
        }
        server {
                listen       8000;
                server_name  localhost;
                location ~ /godfs/group(\d) { 
                    #统一在url前增加godfs,以便统一出入口。
                    proxy_set_header Host $host;
                    proxy_set_header X-Real-IP $remote_addr;
                    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; 
                    rewrite ^/godfs/group(\d) /group$1 break;
                    proxy_pass http://gofastdfs-group$1;
                }
                location ~ /godfs/upload { 
                    #这是一个横向扩展配置，前期可能只有一个集群group1,当group1满后，只需将上传指向group2,
                    #也就是将rewrite , proxy_pass 中的group1改为group2即可。
                    proxy_set_header Host $host;
                    proxy_set_header X-Real-IP $remote_addr;
                    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; 
                    rewrite ^/godfs/upload /group1/upload break;
                    proxy_pass http://gofastdfs-group1;
                }
                location ~ /godfs/big/upload { 
                    #以上上类似。
                    proxy_set_header Host $host;
                    proxy_set_header X-Real-IP $remote_addr;
                    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; 
                    rewrite ^/godfs/upload /group1/big/upload break;
                    proxy_pass http://gofastdfs-group1;
                }

        }
}
```