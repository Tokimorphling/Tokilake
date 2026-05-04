use criterion::{criterion_group, criterion_main, Criterion, Throughput};
use tokio::net::{TcpListener, TcpStream};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokilake_smux::{Config, Session};
use tokio::sync::oneshot;

async fn get_tcp_connection_pair() -> (TcpStream, TcpStream) {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    
    let (tx, rx) = oneshot::channel();
    tokio::spawn(async move {
        let (conn, _) = listener.accept().await.unwrap();
        let _ = tx.send(conn);
    });
    
    let conn1 = TcpStream::connect(addr).await.unwrap();
    let conn0 = rx.await.unwrap();
    (conn0, conn1)
}

async fn get_smux_stream_pair() -> (tokilake_smux::Stream, tokilake_smux::Stream) {
    let (conn0, conn1) = get_tcp_connection_pair().await;
    
    let mut server = Session::server(conn0, Config::default());
    let mut client = Session::client(conn1, Config::default());
    
    let (tx, rx) = oneshot::channel();
    tokio::spawn(async move {
        let server_stream = server.accept().await.unwrap();
        let _ = tx.send(server_stream);
        // Keep server running to process underlying frames
        loop {
            tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
        }
    });
    
    let client_stream = client.open().await.unwrap();
    let server_stream = rx.await.unwrap();
    
    tokio::spawn(async move {
        // Keep client running to process underlying frames
        loop {
            tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
        }
    });
    
    (client_stream, server_stream)
}

fn bench_conn_tcp(c: &mut Criterion) {
    let mut group = c.benchmark_group("BenchmarkConnTCP");
    let chunk_size = 128 * 1024;
    group.throughput(Throughput::Bytes(chunk_size as u64));
    
    let rt = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap();
        
    group.bench_function("tcp", |b| {
        b.to_async(&rt).iter_custom(|iters| async move {
            let (mut conn0, mut conn1) = get_tcp_connection_pair().await;
            
            let iters_usize = iters as usize;
            let start = std::time::Instant::now();
            
            tokio::spawn(async move {
                let mut buf = vec![0u8; chunk_size];
                let mut count = 0;
                while count < chunk_size * iters_usize {
                    let n = conn0.read(&mut buf).await.unwrap();
                    count += n;
                }
            });
            
            let buf = vec![0u8; chunk_size];
            for _ in 0..iters {
                conn1.write_all(&buf).await.unwrap();
            }
            
            start.elapsed()
        })
    });
    group.finish();
}

fn bench_conn_smux(c: &mut Criterion) {
    let mut group = c.benchmark_group("BenchmarkConnSmux");
    let chunk_size = 128 * 1024;
    group.throughput(Throughput::Bytes(chunk_size as u64));
    
    let rt = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap();
        
    group.bench_function("smux", |b| {
        b.to_async(&rt).iter_custom(|iters| async move {
            let (mut stream0, mut stream1) = get_smux_stream_pair().await;
            
            let iters_usize = iters as usize;
            let start = std::time::Instant::now();
            
            tokio::spawn(async move {
                let mut buf = vec![0u8; chunk_size];
                let mut count = 0;
                while count < chunk_size * iters_usize {
                    let n = stream0.read(&mut buf).await.unwrap();
                    if n == 0 {
                        break;
                    }
                    count += n;
                }
            });
            
            let buf = vec![0u8; chunk_size];
            for _ in 0..iters {
                stream1.write_all(&buf).await.unwrap();
            }
            
            start.elapsed()
        })
    });
    group.finish();
}

fn bench_accept_close(c: &mut Criterion) {
    let mut group = c.benchmark_group("BenchmarkAcceptClose");
    
    let rt = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .unwrap();
        
    group.bench_function("accept_close", |b| {
        b.to_async(&rt).iter_custom(|iters| async move {
            let (conn0, conn1) = get_tcp_connection_pair().await;
            
            let mut server = Session::server(conn0, Config::default());
            let mut client = Session::client(conn1, Config::default());
            
            tokio::spawn(async move {
                while let Some(mut stream) = server.accept().await {
                    stream.close().await.unwrap();
                }
            });
            
            let start = std::time::Instant::now();
            for _ in 0..iters {
                if let Some(mut stream) = client.open().await {
                    stream.close().await.unwrap();
                }
            }
            
            start.elapsed()
        })
    });
    group.finish();
}

criterion_group!(benches, bench_conn_tcp, bench_conn_smux, bench_accept_close);
criterion_main!(benches);
