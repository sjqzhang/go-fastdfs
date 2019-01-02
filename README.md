# 简单文件服务器
- 支持curl命令上传
- 支持浏览器上传
- 支持下载
- 支持多机自动同步
- 类fastdfs
- 高性能
- 高可靠

# 优点

- 无依赖(单一文件）
- 自动同步
- 失败自动修复
- 按天分目录方便维护
- 文件去重



# 启动服务器

`./fileserver`

# 配置  (conf/cfg.json)
```json
{
  "addr": ":8080",
  "peers":["http://10.1.50.xx:8080","http://10.1.14.xx:8080","http://10.1.50.xx:8080"],
  "group":"group1",
  "refresh_interval":120,
  "rename_file":false,
  "show_dir":true
}
```


# 上传命令

`curl -F file=@http-index-fs http://10.1.50.90:8080/upload -F "name=http-index-fs" -F "md5=9412f6e58baa25550ab8b34e9778ffd4"` 	
```
说明:  
	name=目标名称(选填)
	md5=文件的md5(选填)
```
