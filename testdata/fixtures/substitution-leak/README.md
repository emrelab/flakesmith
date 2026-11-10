Original, dependency-free reduction of the custom substitution cleanup pattern
in gosimple/slug#83. This models the mechanism without copying upstream code;
it is not counted as real-world evidence. The custom test leaks its setting
into the default substitution test. See docs/real-world/README.md for the
unmodified upstream tests and their before/after validation.
