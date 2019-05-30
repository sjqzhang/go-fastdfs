## 注意点：
```
	一、如果不知道文件大小时，需设置
	    client_max_body_size 0;
	二、如果要开启tus，并使用nginx反向代理,需要设置proxy_redirect
		并且重定向
		proxy_redirect ~/(\w+)/big/upload/(.*) /$1/big/upload/$2;  #继点续传一定要设置(注意)
```
