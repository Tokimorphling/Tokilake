use faststr::FastStr;
pub fn multiline_text(input: &str) -> FastStr {
    // let trie = Trie::from_iter(["a", "app", "apple", "better", "application"]);
    input
        .split('\n')
        .enumerate()
        .map(|(i, v)| {
            if i == 0 {
                v.to_string()
            } else {
                format!(".. {v}")
            }
        })
        .collect::<Vec<String>>()
        .join("\n")
        .into()
}
