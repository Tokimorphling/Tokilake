# `@tokilake/tokiame`

`tokiame` 的 npm 包只做一件事：在安装时下载对应平台的预编译二进制，并把它暴露成 `tokiame` 命令。

## 安装

```bash
npm install -g @tokilake/tokiame
```

## 默认配置路径

安装完成后，`tokiame` 默认读取：

```text
~/.tokilake/tokiame.json
```

安装脚本还会额外写入：

```text
~/.tokilake/tokiame.json.example
```

你可以直接复制一份开始修改：

```bash
cp ~/.tokilake/tokiame.json.example ~/.tokilake/tokiame.json
```

你也可以通过下面两种方式覆盖：

- `TOKIAME_CONFIG=/path/to/tokiame.json`
- `tokiame --config /path/to/tokiame.json`

## 可选环境变量

- `TOKIAME_RELEASE_REPO`: 覆盖 GitHub Releases 仓库，默认 `Tokimorphling/Tokilake`
- `TOKIAME_RELEASE_TAG`: 覆盖 GitHub Releases tag，默认 `v<package-version>`
