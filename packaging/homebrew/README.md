# Homebrew Packaging

`tokiame` 的 Homebrew 分发建议放在单独的 tap 仓库里，例如 `homebrew-tokilake`。

这个目录只保留公式生成脚本，版本和 `sha256` 需要在 release 完成后用对应产物的校验值渲染：

```bash
./hack/scripts/render-tokiame-homebrew-formula.sh \
  --version 1.2.3 \
  --darwin-amd64 <sha256> \
  --darwin-arm64 <sha256> \
  --linux-amd64 <sha256> \
  --linux-arm64 <sha256> \
  > packaging/homebrew/tokiame.rb
```

生成后的 `tokiame.rb` 可以：

- 提交到独立 tap 仓库
- 或直接通过 `brew install --formula https://raw.githubusercontent.com/<repo>/.../tokiame.rb` 安装
