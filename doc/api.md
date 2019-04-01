## API通用说明
```
一、统一使用POST请求
二、返回格式统一为json
　　格式如下
    {
	  "status":"ok",
	  "message":"",
	  "data":{}
	}
二、url中的group只有在support_group_manage设置为true才有。
	例如：
	http://10.1.5.9:8080/group/reload
	默认：
	http://10.1.5.9:8080/reload
	说明：url中的group为cfg.json中的group参数值。

```

## 配置管理API
```
http://10.1.5.9:8080/group/reload

参数：
action: set(修改参数),get获取参数,reload重新加载参数
cfg:json参数　与 action=set配合完成参数设置

```

## 文件统计信息API
```
http://10.1.50.90:8080/group/stat

```

## 文件上传API
```
http://10.1.50.90:8080/group/upload
参数：
file:上传的文件
scene:场景
output:输出
path:自定义路径
具体请参阅示例代码
```

## 文件秒传
```
http://10.1.50.90:8080/group/upload
参数：
md5:文件的摘要
摘要算法要与cfg.json中配置的一样
```


## 文件删除
```
http://10.1.50.90:8080/group/delete
参数：
md5:文件的摘要（md5|sha1） 视配置定
path:文件路径
md5与path二选一
说明：md5或path都是上传文件时返回的信息，要以json方式返回才能看到（参阅浏览器上传）
```

## 修复统计信息
```
http://10.1.50.90:8080/group/repair_stat
```

## 同步失败修复
```
http://10.1.50.90:8080/group/repair
参数：
force:是否强行修复(0|1)
```

## 从文件目录中修复元数据（性能较差）
```
http://10.1.50.90:8080/group/repair_fileinfo

```