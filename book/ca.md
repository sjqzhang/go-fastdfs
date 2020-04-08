
# 注意：先切到conf目录执行以下步骤，中途有输入的请按提示进行输入
## 1.key的生成 
```
openssl genrsa -des3 -out server.key 2048
openssl rsa -in server.key -out server.key
```

## 2. 生成CA的crt
```
openssl req -new -x509 -key server.key -out ca.crt -days 3650
```
## 3. csr的生成方法
```
openssl req -new -key server.key -out server.csr
```

## 4. crt生成方法
```
openssl x509 -req -days 3650 -in server.csr -CA ca.crt -CAkey server.key -CAcreateserial -out server.crt
```