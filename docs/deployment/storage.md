---
title: "存储配置"
layout: doc
outline: deep
lastUpdated: true
---

# 存储配置

因为图片生成有些供应商不提供 url，所以如果设置了存储，那么会上传到存储后返回链接。异步视频生成也可以使用对象存储保存完成后的视频结果，避免用户下载时反复经过 Tokilake 和 Tokiame。

在使用`gemini`支持图像输出的模型时，如果图床设置不正确，会导致图片返回失败。（gemini 原生 API 接口不需要配置）

图片可以设置多个存储，上传失败后会自动使用下一个存储。视频结果使用独立的 `storage.object`，统一走 S3-compatible 协议；Cloudflare R2、MinIO、AWS S3、AliOSS S3 Endpoint 都放在这一套配置里。

```yaml
storage: # 存储设置 (可选,用于图片生成和异步视频结果落地)
  video: # 异步视频结果落地设置
    enabled: false # 开启后，视频完成时会上传到 storage.object，并在下载时跳转到对象存储 URL
    prefix: "videos" # 对象 key 前缀
    max_size_mb: 1024 # 单个视频最大上传大小，超过后保留原有 Tokiame 内容接口
  object: # 异步视频对象存储，统一使用 S3-compatible 协议
    provider: "s3_compatible" # 可填写 s3_compatible、r2、minio、alioss 等标识
    endpoint: "" # Endpoint，比如 https://xxxxxx.r2.cloudflarestorage.com 或 https://oss-cn-beijing.aliyuncs.com
    region: "auto" # R2 可用 auto；AWS/AliOSS 建议填写真实 region
    cdnurl: "" # 公共访问域名；不填则生成临时签名下载 URL
    bucketName: "" # Bucket 名称
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    sessionToken: "" # 可选，临时凭据 token
    forcePathStyle: false # MinIO/R2 常用 true；AliOSS/AWS 通常 false
    presign_ttl_seconds: 3600 # 未配置 cdnurl 时，签名 URL 默认有效期
  smms: # sm.ms 图床设置
    secret: "" # 你的 sm.ms API 密钥
  imgur:
    client_id: "" # 你的 imgur client_id
  alioss: # 阿里云OSS对象存储
    endpoint: "" # Endpoint（地域节点）,比如oss-cn-beijing.aliyuncs.com
    bucketName: "" # Bucket名称，比如zerodeng-superai
    accessKeyId: "" # 阿里授权KEY,在阿里云后台用户RAM控制部分获取
    accessKeySecret: "" # 阿里授权SECRET,在阿里云后台用户RAM控制部分获取
  s3: # AwsS3协议
    endpoint: "" # Endpoint（地域节点）,比如https://xxxxxx.r2.cloudflarestorage.com
    cdnurl: "" # 公共访问域名，比如https://pub-xxxxx.r2.dev，如果不配置则使用endpoint
    bucketName: "" # Bucket名称，比如zerodeng-superai
    accessKeyId: "" # accessKeyId
    accessKeySecret: "" # accessKeySecret
    expirationDays: 3
```
