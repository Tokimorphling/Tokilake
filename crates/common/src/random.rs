use rand::rng;
use rand::rngs::ThreadRng;
use rand::seq::IndexedRandom;

pub struct RandomElementIterator<'a, T> {
    slice: &'a [T],
    rng:   ThreadRng,
}

impl<'a, T> Iterator for RandomElementIterator<'a, T> {
    type Item = &'a T;
    fn next(&mut self) -> Option<Self::Item> {
        if self.slice.is_empty() {
            None
        } else {
            self.slice.choose(&mut self.rng)
        }
    }
}

impl<'a, T> RandomElementIterator<'a, T> {
    pub fn new(slice: &'a [T]) -> Self {
        RandomElementIterator { slice, rng: rng() }
    }
}

pub trait ToRandomIterator<'a, T: 'a> {
    fn random_iter(&'a self) -> RandomElementIterator<'a, T>;
}

impl<'a, T: 'a> ToRandomIterator<'a, T> for Vec<T> {
    fn random_iter(&'a self) -> RandomElementIterator<'a, T> {
        RandomElementIterator::new(self.as_slice())
    }
}

impl<'a, T: 'a> ToRandomIterator<'a, T> for [T] {
    fn random_iter(&'a self) -> RandomElementIterator<'a, T> {
        RandomElementIterator::new(self)
    }
}

#[cfg(test)]
mod tests {
    // Import items from the parent module (or crate root if this code is in lib.rs)
    use super::*;
    use std::{collections::HashMap, vec}; // For the distribution test

    #[test]
    fn test_empty() {
        let empty_vec = Vec::<u32>::new();

        assert!(empty_vec.random_iter().next().is_none());
    }
    #[test]
    fn test_vec_random_iter() {
        let my_vec = vec![10, 20, 30, 40, 50];
        assert!(!my_vec.is_empty());

        // Test .next()
        let mut random_iter_single = my_vec.random_iter();
        let opt_val = random_iter_single.next();
        assert!(
            opt_val.is_some(),
            "Should get Some(element) from non-empty vec"
        );
        assert!(
            my_vec.contains(opt_val.unwrap()),
            "Element should be from the original vec"
        );

        // Test multiple .next() calls
        let mut random_iter_multiple = my_vec.random_iter();
        for _ in 0..5 {
            // Iterate a few times
            let item = random_iter_multiple.next();
            assert!(
                item.is_some(),
                "Should consistently get Some from non-empty vec"
            );
            assert!(
                my_vec.contains(item.unwrap()),
                "Each element should be from the original vec"
            );
        }

        // Test .take() and .collect()
        let count = 3;
        let random_elements: Vec<&i32> = my_vec.random_iter().take(count).collect();
        assert_eq!(
            random_elements.len(),
            count,
            "Collected vector should have the specified count"
        );
        for &val_ref in random_elements.iter() {
            assert!(
                my_vec.contains(val_ref),
                "All collected elements should be from the original vec"
            );
        }
    }

    #[test]
    fn test_array_random_iter() {
        let my_array: [i32; 5] = [100, 200, 300, 400, 500];
        assert!(!my_array.is_empty());

        // Test .next()
        let mut random_iter_single = my_array.random_iter();
        let opt_val = random_iter_single.next();
        assert!(
            opt_val.is_some(),
            "Should get Some(element) from non-empty array"
        );
        assert!(
            my_array.contains(opt_val.unwrap()),
            "Element should be from the original array"
        );

        // Test .take() and .collect()
        let count = 3;
        let random_elements: Vec<&i32> = my_array.random_iter().take(count).collect();
        assert_eq!(random_elements.len(), count);
        for &val_ref in random_elements.iter() {
            assert!(
                my_array.contains(val_ref),
                "All collected elements should be from the original array"
            );
        }
    }

    #[test]
    fn test_slice_random_iter() {
        let source_vec = [10, 20, 30, 40, 50, 60];
        let my_slice: &[i32] = &source_vec[1..4]; // Represents [20, 30, 40]
        assert!(!my_slice.is_empty());
        assert_eq!(my_slice.len(), 3);

        // Test .next()
        let mut random_iter_single = my_slice.random_iter();
        let opt_val = random_iter_single.next();
        assert!(
            opt_val.is_some(),
            "Should get Some(element) from non-empty slice"
        );
        assert!(
            my_slice.contains(opt_val.unwrap()),
            "Element should be from the original slice"
        );

        // Test .take() and .collect()
        // Taking more elements than slice length (with replacement)
        let count = 5;
        let random_elements: Vec<&i32> = my_slice.random_iter().take(count).collect();
        assert_eq!(random_elements.len(), count);
        for &val_ref in random_elements.iter() {
            assert!(
                my_slice.contains(val_ref),
                "All collected elements should be from the original slice"
            );
        }
    }

    #[test]
    fn test_empty_collections_return_none() {
        let empty_vec: Vec<char> = vec![];
        assert!(
            empty_vec.random_iter().next().is_none(),
            "random_iter on empty vec should yield None"
        );

        let empty_array: [char; 0] = [];
        assert!(
            empty_array.random_iter().next().is_none(),
            "random_iter on empty array should yield None"
        );

        let empty_slice_from_vec: &[char] = &empty_vec[..];
        assert!(
            empty_slice_from_vec.random_iter().next().is_none(),
            "random_iter on empty slice should yield None"
        );
    }

    #[test]
    fn test_single_element_collection_always_returns_element() {
        let single_vec = vec![42];
        let mut iter_vec = single_vec.random_iter();
        assert_eq!(
            iter_vec.next(),
            Some(&42),
            "Failed for single element vec (1st draw)"
        );
        assert_eq!(
            iter_vec.next(),
            Some(&42),
            "Failed for single element vec (2nd draw)"
        );

        let single_array = [99];
        let mut iter_array = single_array.random_iter();
        assert_eq!(
            iter_array.next(),
            Some(&99),
            "Failed for single element array (1st draw)"
        );
        assert_eq!(
            iter_array.next(),
            Some(&99),
            "Failed for single element array (2nd draw)"
        );

        let single_slice_source = [7];
        let single_slice: &[i32] = &single_slice_source[..];
        let mut iter_slice = single_slice.random_iter();
        assert_eq!(
            iter_slice.next(),
            Some(&7),
            "Failed for single element slice (1st draw)"
        );
        assert_eq!(
            iter_slice.next(),
            Some(&7),
            "Failed for single element slice (2nd draw)"
        );
    }

    #[test]
    fn test_randomness_distribution_sanity_check() {
        // This test checks if all elements are picked at least once over many samples.
        // It's a probabilistic sanity check, not a guarantee of perfect distribution.
        let my_vec = vec![0, 1, 2, 3, 4];
        if my_vec.len() <= 1 {
            // Not meaningful for single element or empty vecs (covered by other tests)
            return;
        }

        let num_samples = my_vec.len() * 50; // Heuristic: enough samples
        let mut counts = HashMap::new();

        for _ in 0..num_samples {
            if let Some(val_ref) = my_vec.random_iter().next() {
                *counts.entry(*val_ref).or_insert(0u32) += 1;
            } else {
                panic!("Should always get an element from a non-empty vec in distribution test");
            }
        }

        // Check if all original elements were encountered
        for val_in_vec in my_vec.iter() {
            assert!(
                counts.contains_key(val_in_vec) && counts[val_in_vec] > 0,
                "Element {val_in_vec} from original vector was not found in {num_samples} random \
                 samples. Counts: {counts:?}"
            );
        }
        // For debugging/observation:
        // println!("Distribution test counts for {:?}: {:?}", my_vec, counts);
    }
}
