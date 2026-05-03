//! TunnelSession / TunnelStream implementations for tokilake-smux.

use super::{TunnelError, TunnelSession, TunnelStream};
use std::future::Future;

// Implement TunnelSession for tokilake_smux::Session
impl TunnelSession for tokilake_smux::Session {
    type Stream = tokilake_smux::Stream;

    async fn accept_stream(&mut self) -> Result<Option<Self::Stream>, TunnelError> {
        Ok(self.accept().await)
    }

    async fn open_stream(&mut self) -> Result<Self::Stream, TunnelError> {
        self.open().await.ok_or(TunnelError::StreamClosed)
    }

    async fn close(&self) -> Result<(), TunnelError> {
        self.close();
        Ok(())
    }

    fn is_alive(&self) -> bool {
        !self.is_closed()
    }
}

#[allow(clippy::manual_async_fn)]
impl TunnelStream for tokilake_smux::Stream {
    fn read<'a>(
        &'a mut self,
        buf: &'a mut [u8],
    ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a {
        use crate::error::MapTunnelErr;
        async move { self.read(buf).await.map_stream_closed() }
    }

    fn write<'a>(
        &'a mut self,
        buf: &'a [u8],
    ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a {
        use crate::error::MapTunnelErr;
        async move { self.write(buf).await.map_stream_closed() }
    }

    fn flush(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_ {
        async move { Ok(()) }
    }

    fn close(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_ {
        use crate::error::MapTunnelErr;
        async move { self.close().await.map_stream_closed() }
    }
}
