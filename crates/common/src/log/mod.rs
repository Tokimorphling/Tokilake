use tracing_subscriber::{EnvFilter, filter::LevelFilter, fmt, prelude::*};

pub fn logging_stdout() -> impl Drop {
    let (nonblocking, _guard) = tracing_appender::non_blocking(std::io::stdout());

    let default_level = if cfg!(debug_assertions) {
        LevelFilter::DEBUG
    } else {
        LevelFilter::INFO 
    };
    tracing_subscriber::registry()
        // .with(console_subscriber::spawn())
        .with(
            fmt::layer()
                .with_writer(nonblocking)
                .with_file(cfg!(debug_assertions))
                .with_line_number(cfg!(debug_assertions)),
        )
        .with(
            EnvFilter::builder()
                .with_default_directive(default_level.into())
                .from_env_lossy(),
        )
        .init();

    _guard
}
