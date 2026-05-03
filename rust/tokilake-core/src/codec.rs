use crate::{
    error::TunnelError,
    protocol::{ControlMessage, TunnelRequest, TunnelResponse},
};
use tokio::io::{AsyncRead, AsyncWrite, AsyncWriteExt};

/// Control plane codec - newline-delimited JSON.
pub struct ControlCodec<R, W> {
    reader:   R,
    writer:   W,
    read_buf: Vec<u8>,
}

impl<R, W> ControlCodec<R, W>
where
    R: AsyncRead + Unpin,
    W: AsyncWrite + Unpin,
{
    pub fn new(reader: R, writer: W) -> Self {
        Self {
            reader,
            writer,
            read_buf: Vec::with_capacity(4096),
        }
    }

    pub async fn read_message(&mut self) -> Result<Option<ControlMessage>, TunnelError> {
        loop {
            // Check if we have a complete line in the buffer
            if let Some(pos) = self.read_buf.iter().position(|&b| b == b'\n') {
                let line_bytes = self.read_buf[..pos].to_vec();
                self.read_buf.drain(..=pos);

                let line = String::from_utf8_lossy(&line_bytes);
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    continue;
                }

                // Log the first 200 bytes of the message for debugging
                let preview = if trimmed.len() > 200 {
                    &trimmed[..200]
                } else {
                    trimmed
                };
                tracing::debug!(
                    "parsing control message ({} bytes): {}",
                    trimmed.len(),
                    preview
                );

                match serde_json::from_str::<ControlMessage>(trimmed) {
                    Ok(msg) => {
                        tracing::debug!("parsed control message: type={}", msg.msg_type);
                        return Ok(Some(msg));
                    }
                    Err(e) => {
                        tracing::warn!(
                            "failed to parse control message: {} - data: {}",
                            e,
                            preview
                        );
                        return Err(e.into());
                    }
                }
            }

            // Read more data
            let mut buf = [0u8; 4096];
            let n = tokio::io::AsyncReadExt::read(&mut self.reader, &mut buf).await?;
            if n == 0 {
                if self.read_buf.is_empty() {
                    return Ok(None);
                }
                // Try to parse remaining data as a message
                let line_bytes = self.read_buf.clone();
                self.read_buf.clear();
                let line = String::from_utf8_lossy(&line_bytes);
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    return Ok(None);
                }
                let msg: ControlMessage = serde_json::from_str(trimmed)?;
                return Ok(Some(msg));
            }

            tracing::debug!(
                "read {} bytes from control stream, buffer size: {}",
                n,
                self.read_buf.len()
            );
            self.read_buf.extend_from_slice(&buf[..n]);
        }
    }

    pub async fn write_message(&mut self, msg: &ControlMessage) -> Result<(), TunnelError> {
        let mut data = serde_json::to_vec(msg)?;
        data.push(b'\n');
        self.writer.write_all(&data).await?;
        self.writer.flush().await?;
        Ok(())
    }
}

/// Tunnel data plane codec - newline-delimited JSON.
pub struct TunnelCodec<R, W> {
    reader:   R,
    writer:   W,
    read_buf: Vec<u8>,
}

impl<R, W> TunnelCodec<R, W>
where
    R: AsyncRead + Unpin,
    W: AsyncWrite + Unpin,
{
    pub fn new(reader: R, writer: W) -> Self {
        Self {
            reader,
            writer,
            read_buf: Vec::with_capacity(4096),
        }
    }

    pub async fn read_request(&mut self) -> Result<Option<TunnelRequest>, TunnelError> {
        loop {
            if let Some(pos) = self.read_buf.iter().position(|&b| b == b'\n') {
                let line_bytes = self.read_buf[..pos].to_vec();
                self.read_buf.drain(..=pos);

                let line = String::from_utf8_lossy(&line_bytes);
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    continue;
                }

                let req: TunnelRequest = serde_json::from_str(trimmed)?;
                return Ok(Some(req));
            }

            let mut buf = [0u8; 4096];
            let n = tokio::io::AsyncReadExt::read(&mut self.reader, &mut buf).await?;
            if n == 0 {
                if self.read_buf.is_empty() {
                    return Ok(None);
                }
                let line_bytes = self.read_buf.clone();
                self.read_buf.clear();
                let line = String::from_utf8_lossy(&line_bytes);
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    return Ok(None);
                }
                let req: TunnelRequest = serde_json::from_str(trimmed)?;
                return Ok(Some(req));
            }

            self.read_buf.extend_from_slice(&buf[..n]);
        }
    }

    pub async fn write_request(&mut self, req: &TunnelRequest) -> Result<(), TunnelError> {
        let mut data = serde_json::to_vec(req)?;
        data.push(b'\n');
        self.writer.write_all(&data).await?;
        self.writer.flush().await?;
        Ok(())
    }

    pub async fn read_response(&mut self) -> Result<Option<TunnelResponse>, TunnelError> {
        loop {
            if let Some(pos) = self.read_buf.iter().position(|&b| b == b'\n') {
                let line_bytes = self.read_buf[..pos].to_vec();
                self.read_buf.drain(..=pos);

                let line = String::from_utf8_lossy(&line_bytes);
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    continue;
                }

                let resp: TunnelResponse = serde_json::from_str(trimmed)?;
                return Ok(Some(resp));
            }

            let mut buf = [0u8; 4096];
            let n = tokio::io::AsyncReadExt::read(&mut self.reader, &mut buf).await?;
            if n == 0 {
                if self.read_buf.is_empty() {
                    return Ok(None);
                }
                let line_bytes = self.read_buf.clone();
                self.read_buf.clear();
                let line = String::from_utf8_lossy(&line_bytes);
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    return Ok(None);
                }
                let resp: TunnelResponse = serde_json::from_str(trimmed)?;
                return Ok(Some(resp));
            }

            self.read_buf.extend_from_slice(&buf[..n]);
        }
    }

    pub async fn write_response(&mut self, resp: &TunnelResponse) -> Result<(), TunnelError> {
        let mut data = serde_json::to_vec(resp)?;
        data.push(b'\n');
        self.writer.write_all(&data).await?;
        self.writer.flush().await?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::duplex;

    #[tokio::test]
    async fn test_control_codec_roundtrip() {
        let (client, server) = duplex(1024);
        let (client_r, client_w) = tokio::io::split(client);
        let (server_r, server_w) = tokio::io::split(server);
        let mut writer = ControlCodec::new(client_r, client_w);
        let mut reader = ControlCodec::new(server_r, server_w);
        let msg = ControlMessage::auth("test-token");
        writer.write_message(&msg).await.unwrap();
        let received = reader.read_message().await.unwrap().unwrap();
        assert_eq!(received.msg_type, "auth");
        assert_eq!(received.auth.unwrap().token, "test-token");
    }

    #[tokio::test]
    async fn test_tunnel_codec_request_roundtrip() {
        let (client, server) = duplex(1024);
        let (client_r, client_w) = tokio::io::split(client);
        let (server_r, server_w) = tokio::io::split(server);
        let mut writer = TunnelCodec::new(client_r, client_w);
        let mut reader = TunnelCodec::new(server_r, server_w);
        let req = TunnelRequest {
            request_id: "test-123".to_string(),
            route_kind: "chat_completions".to_string(),
            method:     "POST".to_string(),
            path:       "/v1/chat/completions".to_string(),
            model:      "gpt-4".to_string(),
            headers:    Default::default(),
            is_stream:  true,
            body:       b"{}".to_vec(),
        };
        writer.write_request(&req).await.unwrap();
        let received = reader.read_request().await.unwrap().unwrap();
        assert_eq!(received.request_id, "test-123");
        assert_eq!(received.model, "gpt-4");
        assert!(received.is_stream);
    }
}
