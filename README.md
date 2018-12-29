# 简单文件服务器
- 支持curl命令上传
- 支持浏览器上传
- 支持下载



# 上传命令
```

curl -F file=@http-index-fs http://10.1.50.90:8080/upload -F "name=http-index-fs" -F "md5=9412f6e58baa25550ab8b34e9778ffd4" 

说明:  
	name=目标名称(选填)
	md5=文件的md5(选填)
	
	
```
