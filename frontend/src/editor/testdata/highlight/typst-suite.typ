// Corpus for frontend/src/editor/highlight.equivalence.test.ts: the 223 files of tests/suite of
// https://github.com/typst/typst at commit 84cca406aafe7f830ed3f37f1cc6f0c8b6b65a03 (Apache-2.0),
// each after a line `//@@ file <path>`. Error cases included on purpose.
//@@ file tests/suite/format_format.typ
--- format-options-in-container paged ---
#block[
  // Error: 4-27 format set rules are not allowed inside of containers
  #set html(pretty: false)
]
//@@ file tests/suite/format_pages.typ
--- format-options-single-page-range paged empty ---
#set pdf(tagged: false, pages: 3)
#set pdf(tagged: false, pages: (from: 3))
#set pdf(tagged: false, pages: (to: 5))
#set pdf(tagged: false, pages: (from: 3, to: 5))

--- format-options-multiple-page-ranges paged empty ---
#set pdf(tagged: false, pages: (5, 7, 9))
#set pdf(tagged: false, pages: ((to: 3), 5, (from: 7, to: 8), (from: 10)))

--- format-options-page-range-untagged-warnings pdf empty ---
// Warning: exporting a page range disables PDF tagging
// Hint: the resulting PDF will be inaccessible
// Hint: set `pdf(tagged: false)` to silence this warning
#set pdf(pages: 1)

--- format-options-page-nr-string eval ---
// Error: 17-20 expected integer, dictionary, array, or none, found string
#set pdf(pages: "3")

--- format-options-page-range-str eval ---
// Error: 17-22 expected integer, dictionary, array, or none, found string
#set pdf(pages: "3-5")

--- format-options-page-nr-zero eval ---
// Error: 17-18 page numbers start at one
#set pdf(pages: 0)

--- format-options-page-range-inverted eval ---
// Error: 17-33 start of page range cannot be larger than end
#set pdf(pages: (from: 5, to: 2))
//@@ file tests/suite/foundations_arguments.typ
// Test arguments.

--- arguments-len eval ---
#test(arguments().len(), 0)
#test(arguments("hello").len(), 1)
#test(arguments(a: "world").len(), 1)
#test(arguments(a: "hey", 14).len(), 2)
#test(arguments(0, 1, a: 2, 3).len(), 4)

--- arguments-at eval ---
#let args = arguments(0, 1, a: 2, 3)
#test(args.at(0), 0)
#test(args.at(1), 1)
#test(args.at(2), 3)
#test(args.at("a"), 2)

--- arguments-field-access eval ---
#let args = arguments(0, 1, a: 2, 3, b: arguments(5, c: 4))
#test(args.a, 2)
#test(args.b, arguments(5, c: 4))
#test(args.b.c, 4)
#test(args.b.at(0), 5)
#test(args.b.at("c"), 4)
#test(args.at("b").c, 4)

--- arguments-at-call eval ---
// Test calling a function in an argument via `.at()`.
#let args = arguments(x => x + 1, func: x => x + 2)
#test(args.at(0)(0), 1)
#test(args.at("func")(0), 2)

--- arguments-at-index-missing eval ---
#let args = arguments(0, 1, a: 2, 3)
// Error: 2-12 no positional argument at index 4 and no default value was specified
#args.at(4)

--- arguments-at-name-missing eval ---
#let args = arguments(0, 1, a: 2, 3)
// Error: 2-14 no named argument "b" and no default value was specified
#args.at("b")

--- arguments-field-missing eval ---
#let args = arguments(0, 1, a: 2, 3)
// Error: 7-8 no named argument "b"
#args.b

--- arguments-field-invalid-syntax eval ---
#let args = arguments(0)
// Error: 11-11 expected comma
#test(args.0, 0)

--- arguments-field-assign eval ---
#{
  let args = arguments(a: 1)
  // Error: 3-7 cannot mutate fields on arguments
  args.a = 2
  test(args.a, 2)
}

--- arguments-at-assign-named eval ---
#{
  let args = arguments(0, a: 1)
  // Error: 3-15 cannot mutate a temporary value
  args.at("a") = 2
  test(args.a, 2)
}

--- arguments-at-assign-pos eval ---
#{
  let args = arguments(0, a: 1)
  // Error: 3-13 cannot mutate a temporary value
  args.at(0) = 2
  test(args.at(0), 2)
}

--- arguments-field-assign-missing eval ---
#{
  let args = arguments(a: 1)
  // Error: 3-7 cannot mutate fields on arguments
  args.b = 2
}

--- arguments-at-assign-missing-name eval ---
#{
  let args = arguments(a: 1)
  // Error: 3-15 cannot mutate a temporary value
  args.at("b") = 2
}

--- arguments-at-assign-missing-index eval ---
#{
  let args = arguments(0, a: 1)
  // Error: 3-13 cannot mutate a temporary value
  args.at(1) = 2
}

--- arguments-plus-sum-join eval ---
#let lhs = arguments(0, "1", key: "value", 3)
#let rhs = arguments(other-key: 4, key: "other value", 3)
#let result = arguments(0, "1", 3, other-key: 4, key: "other value", 3)
#test(lhs + rhs, result)
#test({lhs; rhs}, result)
#test((lhs, rhs).sum(), result)
#test((lhs, rhs).join(), result)

--- arguments-filter eval ---
// Test the `filter` method.
#test(arguments().filter(calc.even), arguments())
#test(arguments(1, a: 2, b: 3, 4).filter(calc.even), arguments(a: 2, 4))
#test(arguments(h: 7, e: 3, l: 2, o: 5, 1).filter(x => x < 5), arguments(e: 3, l: 2, 1))

--- arguments-filter-error eval ---
// Test that errors in the predicate are reported properly.
// Error: 29-34 cannot subtract integer from string
#arguments("a").filter(x => x - 2)

--- arguments-map eval ---
// Test the `map` method.
#test(arguments().map(x => x * 2), arguments())
#test(arguments(2, a: 3).map(x => x * 2), arguments(4, a: 6))

--- arguments-map-error eval ---
// Test that errors in the function are reported properly.
// Error: 26-31 cannot subtract integer from string
#arguments("a").map(x => x - 2)

--- arguments-method-typo eval ---
#let args = arguments(0)
// Error: 2-10 type arguments has no method `att`
#args.att(0)
//@@ file tests/suite/foundations_array.typ
// Test arrays.

--- array-basic-syntax paged ---
#set page(width: 150pt)

// Empty.
#()

// Not an array, just a parenthesized expression.
#(1)

// One item and trailing comma.
#(-1,)

// No trailing comma.
#(true, false)

// Multiple lines and items and trailing comma.
#("1"
    , rgb("002")
    ,)

--- array-bad-token eval ---
// Error: 4-6 unexpected end of block comment
// Hint: 4-6 consider escaping the `*` with a backslash or opening the block comment with `/*`
#(1*/2)

--- array-bad-number-suffix eval ---
// Error: 6-8 invalid number suffix: `u`
#(1, 1u 2)

--- array-leading-comma eval ---
// Error: 3-4 unexpected comma
#(,1)

--- array-incomplete-pair eval ---
// Missing expression makes named pair incomplete, making this an empty array.
// Error: 5 expected expression
#(a:)

--- array-named-pair eval ---
// Named pair after this is already identified as an array.
// Error: 6-10 expected expression, found named pair
#(1, b: 2)

--- array-keyed-pair eval ---
// Keyed pair after this is already identified as an array.
// Error: 6-14 expected expression, found keyed pair
#(1, "key": 2)

--- array-bad-conversion-from-string eval ---
// Error: 8-15 expected array, bytes, or version, found string
#array("hello")

--- spread-into-array eval ---
// Test spreading into array and dictionary.
#{
  let l = (1, 2, 3)
  let r = (5, 6, 7)
  test((..l, 4, ..r), range(1, 8))
  test((..none), ())
}

--- spread-dict-into-array eval ---
// Error: 9-17 cannot spread dictionary into array
#(1, 2, ..(a: 1))

--- array-len eval ---
// Test the `len` method.
#test(().len(), 0)
#test(("A", "B", "C").len(), 3)

--- array-at-lvalue eval ---
// Test lvalue and rvalue access.
#{
  let array = (1, 2)
  array.at(1) += 5 + array.at(0)
  test(array, (1, 8))
}

--- array-first-and-at-lvalue eval ---
// Test different lvalue method.
#{
  let array = (1, 2, 3)
  array.first() = 7
  array.at(1) *= 8
  test(array, (7, 16, 3))
}

--- array-at-out-of-bounds eval ---
// Test rvalue out of bounds.
// Error: 2-17 array index out of bounds (index: 5, len: 3) and no default value was specified
#(1, 2, 3).at(5)

--- array-at-out-of-bounds-negative eval ---
// Error: 2-18 array index out of bounds (index: -4, len: 3) and no default value was specified
#(1, 2, 3).at(-4)

--- array-at-out-of-bounds-lvalue eval ---
// Test lvalue out of bounds.
#{
  let array = (1, 2, 3)
  // Error: 3-14 array index out of bounds (index: 3, len: 3)
  array.at(3) = 5
}

--- array-at-with-default eval ---
// Test default value.
#test((1, 2, 3).at(2, default: 5), 3)
#test((1, 2, 3).at(3, default: 5), 5)

--- array-remove-with-default eval ---
// Test remove with default value.

#{
  let array = (1, 2, 3)
  test(array.remove(2, default: 5), 3)
}

#{
  let array = (1, 2, 3)
  test(array.remove(3, default: 5), 5)
}

--- array-range eval ---
// Test the `range` function.
#test(range(4), (0, 1, 2, 3))
#test(range(1, 4), (1, 2, 3))
#test(range(-4, 2), (-4, -3, -2, -1, 0, 1))
#test(range(10, 5), ())
#test(range(10, step: 3), (0, 3, 6, 9))
#test(range(1, 4, step: 1), (1, 2, 3))
#test(range(1, 8, step: 2), (1, 3, 5, 7))
#test(range(5, 2, step: -1), (5, 4, 3))
#test(range(10, 0, step: -3), (10, 7, 4, 1))

#test(range(inclusive: true, 0, 0), (0,))
#test(range(inclusive: true, -10, -8), (-10, -9, -8))
#test(range(inclusive: true, -2, 4, step: 2), (-2, 0, 2, 4))
#test(range(inclusive: true, 5, 2, step: -1), (5, 4, 3, 2))
#test(range(inclusive: true, 0, -2, step: -1), (0, -1, -2))

// The user should be able to reach these values.
#test(range(int.max - 2, int.max), (int.max - 2, int.max - 1))
#test(range(int.min + 2, int.min, step: -1), (int.min + 2, -int.max))
#test(range(inclusive: true, int.max - 2, int.max), (int.max - 2, int.max - 1, int.max))
#test(range(inclusive: true, int.min + 2, int.min, step: -1), (int.min + 2, -int.max, int.min))

// Stepping would overflow if not caught.
#test(range(2, 3, step: int.max), (2,))
#test(range(-2, -3, step: int.min), (-2,))
#test(range(inclusive: true, int.max - 1, int.max, step: 2), (int.max - 1,))
#test(range(inclusive: true, int.min + 1, int.min, step: -2), (-int.max,))

--- array-range-end-missing eval ---
// Error: 2-9 missing argument: end
#range()

--- array-range-float-invalid eval ---
// Error: 11-14 expected integer, found float
#range(1, 2.0)

--- array-range-bad-step-type eval ---
// Error: 17-22 expected integer, found string
#range(4, step: "one")

--- array-range-step-zero eval ---
// Error: 18-19 number must not be zero
#range(10, step: 0)

--- array-bad-method-lvalue eval ---
// Test bad lvalue.
#let array = (1, 2, 3)
// Error: 3-14 cannot mutate a temporary value
#(array.len() = 4)

--- array-unknown-method-lvalue eval ---
// Test bad lvalue.
#let array = (1, 2, 3)
// Error: 3-13 type array has no method `yolo`
#(array.yolo() = 4)

--- array-negative-indices eval ---
// Test negative indices.
#{
  let array = (1, 2, 3, 4)
  test(array.at(0), 1)
  test(array.at(-1), 4)
  test(array.at(-2), 3)
  test(array.at(-3), 2)
  test(array.at(-4), 1)
}

--- array-first-and-last eval ---
// The `first` and `last` methods.
#test((1,).first(), 1)
#test((2,).last(), 2)
#test((1, 2, 3).first(), 1)
#test((1, 2, 3).last(), 3)
#test((1, 2).first(default: 99), 1)
#test(().first(default: 99), 99)
#test((1, 2).last(default: 99), 2)
#test(().last(default: 99), 99)

--- array-first-empty eval ---
// Error: 2-12 array is empty
#().first()

--- array-last-empty eval ---
// Error: 2-11 array is empty
#().last()

--- array-push-and-pop eval ---
// Test the `push` and `pop` methods.
#{
  let tasks = (a: (1, 2, 3), b: (4, 5, 6))
  test(tasks.at("a").pop(), 3)
  tasks.b.push(7)
  test(tasks.a, (1, 2))
  test(tasks.at("b"), (4, 5, 6, 7))
}

--- array-insert-and-remove eval ---
// Test the `insert` and `remove` methods.
#{
  let array = (0, 1, 2, 4, 5)
  array.insert(3, 3)
  test(array, range(6))
  _ = array.remove(1)
  test(array, (0, 2, 3, 4, 5))
}

--- array-insert-missing-index eval ---
#let numbers = ()
// Error: 2-18 missing argument: index
#numbers.insert()

--- array-slice eval ---
// Test the `slice` method.
#test((1, 2, 3, 4).slice(2), (3, 4))
#test(range(10).slice(2, 6), (2, 3, 4, 5))
#test(range(10).slice(4, count: 3), (4, 5, 6))
#test(range(10).slice(-5, count: 2), (5, 6))
#test((1, 2, 3).slice(-3, count: 3), (1, 2, 3))
#test((1, 2, 3).slice(-1, count: 1), (3,))
#test((1, 2, 3).slice(2, -2), ())
#test((1, 2, 3).slice(-2, 2), (2,))
#test((1, 2, 3).slice(-3, 2), (1, 2))
#test("ABCD".split("").slice(1, -1).join("-"), "A-B-C-D")

--- array-slice-count-end eval ---
// Error: 2-33 `end` and `count` are mutually exclusive
#(1, 2, 3).slice(0, 1, count: 2)

--- array-slice-out-of-bounds eval ---
// Error: 2-30 array index out of bounds (index: 12, len: 10)
#range(10).slice(9, count: 3)

--- array-slice-out-of-bounds-from-back eval ---
// Error: 2-31 array index out of bounds (index: 12, len: 10)
#range(10).slice(-2, count: 4)

--- array-slice-out-of-bounds-negative eval ---
// Error: 2-24 array index out of bounds (index: -4, len: 3)
#(1, 2, 3).slice(0, -4)

--- array-position eval ---
// Test the `position` method.
#test(("Hi", "❤️", "Love").position(s => s == "❤️"), 1)
#test(("Bye", "💘", "Apart").position(s => s == "❤️"), none)
#test(("A", "B", "CDEF", "G").position(v => v.len() > 2), 2)

--- array-filter eval ---
// Test the `filter` method.
#test(().filter(calc.even), ())
#test((1, 2, 3, 4).filter(calc.even), (2, 4))
#test((7, 3, 2, 5, 1).filter(x => x < 5), (3, 2, 1))

--- array-filter-error eval ---
// Test that errors in the predicate are reported properly.
// Error: 21-26 cannot subtract integer from string
#("a",).filter(x => x - 2)

--- array-filter-wrong-return-type eval ---
// Error: 19-29 expected boolean, found integer
#(1, 2, 3).filter(i => i + 1)

--- array-map eval ---
// Test the `map` method.
#test(().map(x => x * 2), ())
#test((2, 3).map(x => x * 2), (4, 6))

--- array-map-error eval ---
// Test that errors in the function are reported properly.
// Error: 18-23 cannot subtract integer from string
#("a",).map(x => x - 2)

--- array-fold eval ---
// Test the `fold` method.
#test(().fold("hi", grid), "hi")
#test((1, 2, 3, 4).fold(0, (s, x) => s + x), 10)

--- array-fold-closure-without-params eval ---
// Error: 20-22 unexpected argument
#(1, 2, 3).fold(0, () => none)

--- array-sum eval ---
// Test the `sum` method.
#test(().sum(default: 0), 0)
#test(().sum(default: []), [])
#test((1, 2, 3).sum(), 6)

--- array-sum-empty eval ---
// Error: 2-10 cannot calculate sum of empty array with no default
#().sum()

--- array-product eval ---
// Test the `product` method.
#test(().product(default: 0), 0)
#test(().product(default: []), [])
#test(([ab], 3).product(), [ab]*3)
#test((1, 2, 3).product(), 6)

--- array-product-empty eval ---
// Error: 2-14 cannot calculate product of empty array with no default
#().product()

--- array-rev eval ---
// Test the `rev` method.
#test(range(3).rev(), (2, 1, 0))

--- array-join eval ---
// Test the `join` method.
#test(().join(), none)
#test((1,).join(), 1)
#test(("a", "b", "c").join(), "abc")
#test("(" + ("a", "b", "c").join(", ") + ")", "(a, b, c)")

--- array-join-default eval ---
#test(().join(default: "EMPTY", ", "), "EMPTY")
#test(("hello",).join(default: "EMPTY", ", "), "hello")
#test(("hello", "world").join(default: "EMPTY", ", "), "hello, world")

--- array-join-bad-values eval ---
// Error: 2-22 cannot join boolean with boolean
#(true, false).join()

--- array-join-bad-separator eval ---
// Error: 2-20 cannot join string with integer
#("a", "b").join(1)

--- array-join-content paged ---
// Test joining content.
#([One], [Two], [Three]).join([, ], last: [ and ]).

--- array-intersperse eval ---
// Test the `intersperse` method
#test(().intersperse("a"), ())
#test((1,).intersperse("a"), (1,))
#test((1, 2).intersperse("a"), (1, "a", 2))
#test((1, 2, "b").intersperse("a"), (1, "a", 2, "a", "b"))

--- array-chunks eval ---
// Test the `chunks` method.
#test(().chunks(10), ())
#test((1, 2, 3).chunks(10), ((1, 2, 3),))
#test((1, 2, 3, 4, 5, 6).chunks(3), ((1, 2, 3), (4, 5, 6)))
#test((1, 2, 3, 4, 5, 6, 7, 8).chunks(3), ((1, 2, 3), (4, 5, 6), (7, 8)))

#test(().chunks(10, exact: true), ())
#test((1, 2, 3).chunks(10, exact: true), ())
#test((1, 2, 3, 4, 5, 6).chunks(3, exact: true), ((1, 2, 3), (4, 5, 6)))
#test((1, 2, 3, 4, 5, 6, 7, 8).chunks(3, exact: true), ((1, 2, 3), (4, 5, 6)))

--- array-chunks-size-zero eval ---
// Error: 19-20 number must be positive
#(1, 2, 3).chunks(0)

--- array-chunks-size-negative eval ---
// Error: 19-21 number must be positive
#(1, 2, 3).chunks(-5)

--- array-windows eval ---
// Test the `windows` method.
#test(().windows(5), ())
#test((1, 2, 3).windows(5), ())
#test((1, 2, 3, 4, 5).windows(3), ((1, 2, 3), (2, 3, 4), (3, 4, 5)))
#test((1, 2, 3, 4, 5, 6, 7, 8).windows(5), ((1, 2, 3, 4, 5), (2, 3, 4, 5, 6), (3, 4, 5, 6, 7), (4, 5, 6, 7, 8)))

--- array-windows-size-zero eval ---
// Error: 20-21 number must be positive
#(1, 2, 3).windows(0)

--- array-windows-size-negative eval ---
// Error: 20-22 number must be positive
#(1, 2, 3).windows(-5)

--- array-sorted eval ---
// Test the `sorted` method.
#test(().sorted(), ())
#test(().sorted(key: x => x), ())
#test(((true, false) * 10).sorted(), (false,) * 10 + (true,) * 10)
#test(("it", "the", "hi", "text").sorted(), ("hi", "it", "text", "the"))
#test(("I", "the", "hi", "text").sorted(key: x => x), ("I", "hi", "text", "the"))
#test(("I", "the", "hi", "text").sorted(key: x => x.len()), ("I", "hi", "the", "text"))
#test((2, 1, 3, 10, 5, 8, 6, -7, 2).sorted(), (-7, 1, 2, 2, 3, 5, 6, 8, 10))
#test((2, 1, 3, -10, -5, 8, 6, -7, 2).sorted(key: x => x), (-10, -7, -5, 1, 2, 2, 3, 6, 8))
#test((2, 1, 3, -10, -5, 8, 6, -7, 2).sorted(key: x => x * x), (1, 2, 2, 3, -5, 6, -7, 8, -10))
#test(("I", "the", "hi", "text").sorted(by: (x, y) => x.len() < y.len()), ("I", "hi", "the", "text"))
#test(("I", "the", "hi", "text").sorted(key: x => x.len(), by: (x, y) => y < x), ("text", "the", "hi", "I"))

--- array-sorted-invalid-by-function eval ---
// Error: 2-39 expected boolean from `by` function, got string
#(1, 2, 3).sorted(by: (_, _) => "hmm")

--- array-sorted-key-function-positional-1 eval ---
// Error: 12-18 unexpected argument
#().sorted(x => x)

--- array-zip eval ---
// Test the `zip` method.
#test(().zip(()), ())
#test((1,).zip(()), ())
#test((1,).zip((2,)), ((1, 2),))
#test((1, 2).zip((3, 4)), ((1, 3), (2, 4)))
#test((1, 2).zip((3, 4), exact: true), ((1, 3), (2, 4)))
#test((1, 2, 3, 4).zip((5, 6)), ((1, 5), (2, 6)))
#test(((1, 2), 3).zip((4, 5)), (((1, 2), 4), (3, 5)))
#test((1, "hi").zip((true, false)), ((1, true), ("hi", false)))
#test((1, 2, 3).zip((3, 4, 5), (6, 7, 8)), ((1, 3, 6), (2, 4, 7), (3, 5, 8)))
#test(().zip((), ()), ())
#test((1,).zip((2,), (3,)), ((1, 2, 3),))
#test((1, 2, 3).zip(), ((1,), (2,), (3,)))
#test(array.zip(()), ())

--- array-zip-exact-error eval ---
// Error: 13-22 second array has different length (3) from first array (2)
#(1, 2).zip((1, 2, 3), exact: true)

--- array-zip-exact-multi-error eval ---
// Error: 13-22 array has different length (3) from first array (2)
// Error: 24-36 array has different length (4) from first array (2)
#(1, 2).zip((1, 2, 3), (1, 2, 3, 4), exact: true)

--- array-enumerate eval ---
// Test the `enumerate` method.
#test(().enumerate(), ())
#test(().enumerate(start: 5), ())
#test(("a", "b", "c").enumerate(), ((0, "a"), (1, "b"), (2, "c")))
#test(("a", "b", "c").enumerate(start: 1), ((1, "a"), (2, "b"), (3, "c")))
#test(("a", "b", "c").enumerate(start: 42), ((42, "a"), (43, "b"), (44, "c")))
#test(("a", "b", "c").enumerate(start: -7), ((-7, "a"), (-6, "b"), (-5, "c")))

--- array-dedup eval ---
// Test the `dedup` method.
#test(().dedup(), ())
#test((1,).dedup(), (1,))
#test((1, 1).dedup(), (1,))
#test((1, 2, 1).dedup(), (1, 2))
#test(("Jane", "John", "Eric").dedup(), ("Jane", "John", "Eric"))
#test(("Jane", "John", "Eric", "John").dedup(), ("Jane", "John", "Eric"))

--- array-dedup-key eval ---
// Test the `dedup` method with the `key` argument.
#test((1, 2, 3, 4, 5, 6).dedup(key: x => calc.rem(x, 2)), (1, 2))
#test((1, 2, 3, 4, 5, 6).dedup(key: x => calc.rem(x, 3)), (1, 2, 3))
#test(("Hello", "World", "Hi", "There").dedup(key: x => x.len()), ("Hello", "Hi"))
#test(("Hello", "World", "Hi", "There").dedup(key: x => x.at(0)), ("Hello", "World", "There"))

--- array-to-dict eval ---
// Test the `to-dict` method.
#test(().to-dict(), (:))
#test((("a", 1), ("b", 2), ("c", 3)).to-dict(), (a: 1, b: 2, c: 3))
#test((("a", 1), ("b", 2), ("c", 3), ("b", 4)).to-dict(), (a: 1, b: 4, c: 3))

--- array-to-dict-bad-item-type eval ---
// Error: 2-16 expected (str, any) pairs, found integer
#(1,).to-dict()

--- array-to-dict-bad-pair-length-1 eval ---
// Error: 2-19 expected pairs of length 2, found length 1
#((1,),).to-dict()

--- array-to-dict-bad-pair-length-3 eval ---
// Error: 2-26 expected pairs of length 2, found length 3
#(("key",1,2),).to-dict()

--- array-to-dict-bad-key-type eval ---
// Error: 2-21 expected key of type string, found integer
#((1, 2),).to-dict()

--- array-to-dict-none-key-type eval ---
// Error: 2-24 expected key of type string, found none
#((none, 2),).to-dict()

--- array-zip-positional-and-named-argument eval ---
// Error: 13-30 unexpected argument: val
#().zip((), val: "applicable")

--- array-sorted-bad-key eval ---
// Error: 32-37 cannot divide by zero
#(1, 2, 0, 3).sorted(key: x => 5 / x)

--- array-sorted-underdetermined-with-key eval ---
// Error: 2-40 cannot compare content and content
// Hint: 2-40 consider defining the comparison with `by` or choosing a different `key`
#((1, []), (1, [])).sorted(key: a => a)

--- array-sorted-uncomparable eval ---
// Error: 2-26 cannot compare content and content
// Hint: 2-26 consider choosing a `key` or defining the comparison with `by`
#([Hi], [There]).sorted()

--- array-sorted-uncomparable-lengths eval ---
// Error: 2-26 cannot compare 3em with 2pt
// Hint: 2-26 consider choosing a `key` or defining the comparison with `by`
#(1pt, 2pt, 3em).sorted()

--- array-sorted-key-function-positional-2 eval ---
// Error: 42-52 unexpected argument
#((k: "a", v: 2), (k: "b", v: 1)).sorted(it => it.v)

--- issue-3014-mix-array-dictionary eval ---
// Error: 8-17 expected expression, found named pair
#(box, fill: red)

--- issue-3154-array-first-empty eval ---
#{
  let array = ()
  // Error: 3-16 array is empty
  array.first()
}

--- issue-3154-array-first-mutable-empty eval ---
#{
  let array = ()
  // Error: 3-16 array is empty
  array.first() = 9
}

--- issue-3154-array-last-empty eval ---
#{
  let array = ()
  // Error: 3-15 array is empty
  array.last()
}

--- issue-3154-array-last-mutable-empty eval ---
#{
  let array = ()
  // Error: 3-15 array is empty
  array.last() = 9
}

--- issue-3154-array-at-out-of-bounds eval ---
#{
  let array = (1,)
  // Error: 3-14 array index out of bounds (index: 1, len: 1) and no default value was specified
  array.at(1)
}

--- issue-3154-array-at-out-of-bounds-default eval ---
#{
  let array = (1,)
  test(array.at(1, default: 0), 0)
}

--- issue-3154-array-at-out-of-bounds-mutable eval ---
#{
  let array = (1,)
  // Error: 3-14 array index out of bounds (index: 1, len: 1)
  array.at(1) = 9
}

--- issue-3154-array-at-out-of-bounds-mutable-default eval ---
#{
  let array = (1,)
  // Error: 3-26 array index out of bounds (index: 1, len: 1)
  array.at(1, default: 0) = 9
}

--- array-unopened eval ---
// Error: 2-3 unclosed delimiter
#{)}

--- array-unclosed eval ---
// Error: 3-4 unclosed delimiter
#{(}

--- array-reduce eval ---
// Test the `reduce` method.
#test(().reduce(grid), none)
#test((1, 2, 3, 4).reduce((s, x) => s + x), 10)

--- array-reduce-missing-reducer eval ---
// Error: 2-13 missing argument: reducer
#().reduce()

--- array-reduce-unexpected-argument eval ---
// Error: 19-21 unexpected argument
#(1, 2, 3).reduce(() => none)

--- issue-6285-crashed-with-sorting-non-total-order eval ---
// Error: 2-66 cannot compare none and string
// Hint: 2-66 consider choosing a `key` or defining the comparison with `by`
#(("a", "b", "c", "d", "e", "z") * 3 + ("c", none, "a")).sorted()
//@@ file tests/suite/foundations_assert.typ
--- assert-fail eval ---
// Test failing assertions.
// Error: 2-16 assertion failed
#assert(1 == 2)

--- assert-fail-message eval ---
// Test failing assertions.
// Error: 2-51 assertion failed: two is smaller than one
#assert(2 < 1, message: "two is smaller than one")

--- assert-bad-type eval ---
// Test failing assertions.
// Error: 9-15 expected boolean, found string
#assert("true")

--- assert-eq-fail eval ---
// Test failing assertions.
// Error: 2-19 equality assertion failed: value 10 was not equal to 11
#assert.eq(10, 11)

--- assert-eq-fail-message eval ---
// Test failing assertions.
// Error: 2-55 equality assertion failed: 10 and 12 are not equal
#assert.eq(10, 12, message: "10 and 12 are not equal")

--- assert-ne-fail eval ---
// Test failing assertions.
// Error: 2-19 inequality assertion failed: value 11 was equal to 11
#assert.ne(11, 11)

--- assert-ne-fail-message eval ---
// Test failing assertions.
// Error: 2-57 inequality assertion failed: must be different from 11
#assert.ne(11, 11, message: "must be different from 11")

--- assert-success eval ---
// Test successful assertions.
#assert(5 > 3)
#assert.eq(15, 15)
#assert.ne(10, 12)
//@@ file tests/suite/foundations_bytes.typ
// Test the bytes type.

--- bytes-basic eval ---
#let data = read("/assets/images/rhino.png", encoding: none)
#test(data.len(), 232243)
#test(data.slice(0, count: 5), bytes((137, 80, 78, 71, 13)))
#test(str(data.slice(1, 4)), "PNG")
#test(repr(data), "bytes(232243)")

--- bytes-string-conversion eval ---
#test(str(bytes(range(0x41, 0x50))), "ABCDEFGHIJKLMNO")

--- bytes-array-conversion eval ---
#test(array(bytes("Hello")), (0x48, 0x65, 0x6C, 0x6C, 0x6F))

--- bytes-addition eval ---
// Test addition and joining.
#test(bytes((1, 2)) + bytes(()), bytes((1, 2)))
#test(bytes((1, 2)) + bytes((3, 4)), bytes((1, 2, 3, 4)))
#test(bytes(()) + bytes((3, 4)), bytes((3, 4)))

--- bytes-joining eval ---
#test(str({
  bytes("Hello")
  bytes((0x20,))
  bytes("World")
}), "Hello World")

--- bytes-bad-conversion-from-dict eval ---
// Error: 8-14 expected string, array, or bytes, found dictionary
#bytes((a: 1))

--- bytes-slice eval ---
// Test the `slice` method.
#test(bytes("abcd").slice(2), bytes("cd"))
#test(bytes("abcd").slice(0, 3), bytes("abc"))
#test(bytes("abcd").slice(1, -1), bytes("bc"))
#test(bytes("abcd").slice(3, 3), bytes(""))
#test(bytes("abcd").slice(3, 0), bytes(""))
#test(bytes("abcd").slice(-2), bytes("cd"))
#test(bytes("abcd").slice(-3, 2), bytes("b"))
#test(bytes("abcd").slice(-3, -1), bytes("bc"))
#test(bytes("abcd").slice(-2, -2), bytes(""))
#test(bytes("abcd").slice(1, count: 3), bytes("bcd"))
#test(bytes("abcd").slice(-3, count: 3), bytes("bcd"))
#test(bytes("abcd").slice(2, count: 0), bytes(""))
//@@ file tests/suite/foundations_calc.typ
--- calc-round eval ---
#test(calc.round(calc.e, digits: 2), 2.72)
#test(calc.round(calc.pi, digits: 2), 3.14)
#test(type(calc.round(3.1415, digits: 2)), float)
#test(type(calc.round(5, digits: 2)), int)
#test(type(calc.round(decimal("3.1415"), digits: 2)), decimal)
#test(type(calc.round(314.15, digits: -2)), float)
#test(type(calc.round(523, digits: -2)), int)
#test(type(calc.round(decimal("314.15"), digits: -2)), decimal)

--- calc-round-large-inputs eval ---
#test(calc.round(31114, digits: 4000000000), 31114)
#test(calc.round(int.max, digits: 12), int.max)
#test(calc.round(int.max, digits: -20), 0)
#test(calc.round(238959235.129590203, digits: 4000000000), 238959235.129590203)
#test(calc.round(1.7976931348623157e+308, digits: 12), 1.7976931348623157e+308)
#test(calc.round(1.7976931348623157e+308, digits: -308), float.inf)
#test(calc.round(-1.7976931348623157e+308, digits: -308), -float.inf)
#test(calc.round(12.34, digits: -312), 0.0)
#test(calc.round(decimal("238959235.129590203"), digits: 4000000000), decimal("238959235.129590203"))
#test(calc.round(decimal("79228162514264337593543950335"), digits: 12), decimal("79228162514264337593543950335"))
#test(calc.round(decimal("79228162514264337593543950335"), digits: -50), decimal("0"))
#test(calc.round(decimal("-79228162514264337593543950335"), digits: -2), decimal("-79228162514264337593543950300"))

--- calc-abs eval ---
// Test the `abs` function.
#test(calc.abs(-3), 3)
#test(calc.abs(3), 3)
#test(calc.abs(-0.0), 0.0)
#test(calc.abs(0.0), -0.0)
#test(calc.abs(-3.14), 3.14)
#test(calc.abs(50%), 50%)
#test(calc.abs(-25%), 25%)
#test(calc.abs(decimal("4932.493249324932")), decimal("4932.493249324932"))
#test(calc.abs(decimal("-12402.593295932041")), decimal("12402.593295932041"))

--- calc-abs-bad-type eval ---
// Error: 11-22 expected integer, float, length, angle, ratio, fraction, or decimal, found string
#calc.abs("no number")

--- calc-abs-too-large eval ---
// Error: 11-18 the result is too large
#calc.abs(int.min)

--- calc-even-and-odd eval ---
// Test the `even` and `odd` functions.
#test(calc.even(2), true)
#test(calc.odd(2), false)
#test(calc.odd(-1), true)
#test(calc.even(-11), false)

--- calc-rem eval ---
// Test the `rem` function.
#test(calc.rem(1, 1), 0)
#test(calc.rem(5, 3), 2)
#test(calc.rem(5, -3), 2)
#test(calc.rem(22.5, 10), 2.5)
#test(calc.rem(9, 4.5), 0)
#test(calc.rem(decimal("5"), -3), decimal("2"))
#test(calc.rem(decimal("22.5"), decimal("10")), decimal("2.5"))
#test(calc.rem(9, decimal("4.5")), decimal("0"))
#test(calc.rem(decimal("7"), decimal("3")), decimal("1"))
#test(calc.rem(decimal("7"), decimal("-3")), decimal("1"))
#test(calc.rem(decimal("-7"), decimal("3")), decimal("-1"))
#test(calc.rem(decimal("-7"), decimal("-3")), decimal("-1"))

// Ensure `i64::MIN % -1` will not overflow and panic.
#test(calc.rem(int.min, -1), 0)
#test(calc.rem(float(int.min), -1.0), 0.0)

--- calc-rem-divisor-zero-1 eval ---
// Error: 14-15 divisor must not be zero
#calc.rem(5, 0)

--- calc-rem-divisor-zero-2 eval ---
// Error: 16-19 divisor must not be zero
#calc.rem(3.0, 0.0)

--- calc-rem-divisor-zero-3 eval ---
// Error: 27-39 divisor must not be zero
#calc.rem(decimal("4.0"), decimal("0"))

--- calc-div-euclid eval ---
// Test the `div-euclid` function.
#test(calc.div-euclid(7, 3), 2)
#test(calc.div-euclid(7, -3), -2)
#test(calc.div-euclid(-7, 3), -3)
#test(calc.div-euclid(-7, -3), 3)
#test(calc.div-euclid(2.5, 2), 1)
#test(calc.div-euclid(decimal("7"), decimal("3")), decimal("2"))
#test(calc.div-euclid(decimal("7"), decimal("-3")), decimal("-2"))
#test(calc.div-euclid(decimal("-7"), decimal("3")), decimal("-3"))
#test(calc.div-euclid(decimal("-7"), decimal("-3")), decimal("3"))
#test(calc.div-euclid(decimal("2.5"), decimal("2")), decimal("1"))

--- calc-div-euclid-divisor-zero-1 eval ---
// Error: 21-22 divisor must not be zero
#calc.div-euclid(5, 0)

--- calc-div-euclid-divisor-zero-2 eval ---
// Error: 23-26 divisor must not be zero
#calc.div-euclid(3.0, 0.0)

--- calc-div-euclid-divisor-zero-3 eval ---
// Error: 35-50 divisor must not be zero
#calc.div-euclid(decimal("3.00"), decimal("0.00"))

--- calc-div-euclid-too-large eval ---
// Error: 2-30 the result is too large
#calc.div-euclid(int.min, -1)

--- calc-rem-euclid eval ---
// Test the `rem-euclid` function.
#test(calc.rem-euclid(7, 3), 1)
#test(calc.rem-euclid(7, -3), 1)
#test(calc.rem-euclid(-7, 3), 2)
#test(calc.rem-euclid(-7, -3), 2)
#test(calc.rem-euclid(2.5, 2), 0.5)
#test(calc.rem-euclid(decimal("7"), decimal("3")), decimal("1"))
#test(calc.rem-euclid(decimal("7"), decimal("-3")), decimal("1"))
#test(calc.rem-euclid(decimal("-7"), decimal("3")), decimal("2"))
#test(calc.rem-euclid(decimal("-7"), decimal("-3")), decimal("2"))
#test(calc.rem-euclid(decimal("2.5"), decimal("2")), decimal("0.5"))

// Ensure `i64::MIN % -1` will not overflow and panic.
#test(calc.rem-euclid(int.min, -1), 0)
#test(calc.rem-euclid(float(int.min), -1.0), 0.0)

--- calc-rem-euclid-divisor-zero-1 eval ---
// Error: 21-22 divisor must not be zero
#calc.rem-euclid(5, 0)

--- calc-rem-euclid-divisor-zero-2 eval ---
// Error: 23-26 divisor must not be zero
#calc.rem-euclid(3.0, 0.0)

--- calc-rem-euclid-divisor-zero-3 eval ---
// Error: 35-50 divisor must not be zero
#calc.rem-euclid(decimal("3.00"), decimal("0.00"))

--- calc-quo eval ---
// Test the `quo` function.
#test(calc.quo(1, 1), 1)
#test(calc.quo(5, 3), 1)
#test(calc.quo(5, -3), -2)
#test(calc.quo(-5, 3), -2)
#test(calc.quo(-5, -3), 1)
#test(calc.quo(6, -3), -2)
#test(calc.quo(-4, 5), -1)
#test(calc.quo(-4, 5.), -1)
#test(calc.quo(22.5, 10), 2)
#test(calc.quo(9, 4.5), 2)
#test(calc.quo(decimal("22.5"), 10), 2)
#test(calc.quo(decimal("9"), decimal("4.5")), 2)
#test(calc.quo(decimal("-9"), decimal("4.1")), -3)

--- calc-quo-divisor-zero-1 eval ---
// Error: 14-15 divisor must not be zero
#calc.quo(5, 0)

--- calc-quo-divisor-zero-2 eval ---
// Error: 16-19 divisor must not be zero
#calc.quo(3.0, 0.0)

--- calc-quo-divisor-zero-3 eval ---
// Error: 27-41 divisor must not be zero
#calc.quo(decimal("4.0"), decimal("0.0"))

--- calc-quo-too-large eval ---
// Error: 2-23 the result is too large
#calc.quo(int.min, -1)

--- calc-min-and-max eval ---
// Test the `min` and `max` functions.
#test(calc.min(2, -4), -4)
#test(calc.min(3.5, 1e2, -0.1, 3), -0.1)
#test(calc.min(decimal("3.5"), 4, decimal("-3213.99999")), decimal("-3213.99999"))
#test(calc.max(-3, 11), 11)
#test(calc.max(decimal("3"), 45), 45)
#test(calc.min("hi"), "hi")

--- calc-pow-log-exp-ln eval ---
// Test the `pow`, `log`, `exp`, and `ln` functions.
#test(calc.pow(10, 0), 1)
#test(calc.pow(2, 4), 16)
#test(calc.pow(decimal("0.5"), 18), decimal("0.000003814697265625"))
#test(calc.exp(2), calc.pow(calc.e, 2))
#test(calc.ln(10), calc.log(10, base: calc.e))

--- calc-erf eval ---
// Test the `erf` function.
#test(calc.erf(0), 0)
#test(calc.erf(1), 0.8427007929497149)
#test(calc.erf(calc.inf), 1)
#test(calc.erf(-1), -calc.erf(1))

--- calc-bit-logical eval ---
// Test the `bit-not`, `bit-and`, `bit-or` and `bit-xor` functions.
#test(64.bit-not(), -65)
#test(0.bit-not(), -1)
#test((-56).bit-not(), 55)
#test(128.bit-and(192), 128)
#test(192.bit-and(224), 192)
#test((-1).bit-and(325532), 325532)
#test(0.bit-and(-53), 0)
#test(0.bit-or(-1), -1)
#test(5.bit-or(3), 7)
#test((-50).bit-or(3), -49)
#test(64.bit-or(32), 96)
#test((-1).bit-xor(1), -2)
#test(64.bit-xor(96), 32)
#test((-1).bit-xor(-7), 6)
#test(0.bit-xor(492), 492)

--- calc-bit-shift eval ---
// Test the `bit-lshift` and `bit-rshift` functions.
#test(32.bit-lshift(2), 128)
#test(694.bit-lshift(0), 694)
#test(128.bit-rshift(2), 32)
#test(128.bit-rshift(12345), 0)
#test((-7).bit-rshift(2), -2)
#test((-7).bit-rshift(12345), -1)
#test(128.bit-rshift(2, logical: true), 32)
#test((-7).bit-rshift(61, logical: true), 7)
#test(128.bit-rshift(12345, logical: true), 0)
#test((-7).bit-rshift(12345, logical: true), 0)

--- calc-bit-shift-too-large eval ---
// Error: 2-18 the result is too large
#1.bit-lshift(64)

--- calc-bit-lshift-negative eval ---
// Error: 15-17 number must be at least zero
#1.bit-lshift(-1)

--- calc-bit-rshift-negative eval ---
// Error: 15-17 number must be at least zero
#1.bit-rshift(-1)

--- calc-pow-zero-to-power-of-zero eval ---
// Error: 2-16 zero to the power of zero is undefined
#calc.pow(0, 0)

--- calc-pow-exponent-too-large eval ---
// Error: 14-31 exponent is too large
#calc.pow(2, 10000000000000000)

--- calc-pow-too-large eval ---
// Error: 2-25 the result is too large
#calc.pow(2, 2147483647)

--- calc-pow-too-large-decimal eval ---
// Error: 2-56 the result is too large
#calc.pow(decimal("2222222222222222222222222222"), 100)

--- calc-pow-bad-exponent eval ---
// Error: 14-36 exponent may not be infinite, subnormal, or NaN
#calc.pow(2, calc.pow(2.0, 10000.0))

--- calc-pow-not-real eval ---
// Error: 2-19 the result is not a real number
#calc.pow(-1, 0.5)

--- calc-sqrt-not-real eval ---
// Error: 12-14 cannot take square root of negative number
#calc.sqrt(-1)

--- calc-root eval ---
#test(calc.root(12.0, 1), 12.0)
#test(calc.root(9.0, 2), 3.0)
#test(calc.root(27.0, 3), 3.0)
#test(calc.root(-27.0, 3), -3.0)
// 100^(-1/2) = (100^(1/2))^-1 = 1/sqrt(100)
#test(calc.root(100.0, -2), 0.1)

--- calc-root-zeroth eval ---
// Error: 17-18 cannot take the 0th root of a number
#calc.root(1.0, 0)

--- calc-root-negative-even eval ---
// Error: 24-25 negative numbers do not have a real nth root when n is even
#test(calc.root(-27.0, 4), -3.0)

--- calc-log-negative eval ---
// Error: 11-13 value must be strictly positive
#calc.log(-1)

--- calc-log-bad-base eval ---
// Error: 20-21 base may not be zero, NaN, infinite, or subnormal
#calc.log(1, base: 0)

--- calc-log-not-real eval ---
// Error: 2-24 the result is not a real number
#calc.log(10, base: -1)

--- calc-fact eval ---
// Test the `fact` function.
#test(calc.fact(0), 1)
#test(calc.fact(5), 120)

--- calc-fact-too-large eval ---
// Error: 2-15 the result is too large
#calc.fact(21)

--- calc-perm eval ---
// Test the `perm` function.
#test(calc.perm(0, 0), 1)
#test(calc.perm(5, 3), 60)
#test(calc.perm(5, 5), 120)
#test(calc.perm(5, 6), 0)

--- calc-perm-too-large eval ---
// Error: 2-19 the result is too large
#calc.perm(21, 21)

--- calc-binom eval ---
// Test the `binom` function.
#test(calc.binom(0, 0), 1)
#test(calc.binom(5, 3), 10)
#test(calc.binom(5, 5), 1)
#test(calc.binom(5, 6), 0)
#test(calc.binom(6, 2), 15)

--- calc-gcd eval ---
// Test the `gcd` function.
#test(calc.gcd(112, 77), 7)
#test(calc.gcd(12, 96), 12)
#test(calc.gcd(13, 9), 1)
#test(calc.gcd(13, -9), 1)
#test(calc.gcd(272557, 272557), 272557)
#test(calc.gcd(0, 0), 0)
#test(calc.gcd(7, 0), 7)

--- calc-gcd-too-large eval ---
// Error: 2-23 the result is too large
#calc.gcd(int.min, -1)

--- calc-gcd-min-int eval ---
// Error: 2-22 the result is too large
#calc.gcd(int.min, 0)

--- calc-lcm eval ---
// Test the `lcm` function.
#test(calc.lcm(112, 77), 1232)
#test(calc.lcm(12, 96), 96)
#test(calc.lcm(13, 9), 117)
#test(calc.lcm(13, -9), 117)
#test(calc.lcm(272557, 272557), 272557)
#test(calc.lcm(0, 0), 0)
#test(calc.lcm(8, 0), 0)

--- calc-lcm-too-large eval ---
// Error: 2-41 the result is too large
#calc.lcm(15486487489457, 4874879896543)

--- calc-lcm-min-int eval ---
// Error: 2-22 the result is too large
#calc.lcm(int.min, 1)

--- calc-round-larger-than-max-int eval ---
#test(calc.round(decimal("9223372036854775809.5")), decimal("9223372036854775810"))
#test(calc.round(9223372036854775809.5), 9223372036854775810.0)

--- calc-floor-float-larger-than-max-int eval ---
// Error: 2-35 the result is too large
#calc.floor(9223372036854775809.5)

--- calc-floor-decimal-larger-than-max-int eval ---
// Error: 2-46 the result is too large
#calc.floor(decimal("9223372036854775809.5"))

--- calc-round-smaller-than-min-int eval ---
#test(calc.round(decimal("-9223372036854775809.5")), decimal("-9223372036854775810"))
#test(calc.round(-9223372036854775809.5), -9223372036854775810.0)

--- calc-floor-float-smaller-than-min-int eval ---
// Error: 2-36 the result is too large
#calc.floor(-9223372036854775809.5)

--- calc-floor-decimal-smaller-than-min-int eval ---
// Error: 2-47 the result is too large
#calc.floor(decimal("-9223372036854775809.5"))

--- calc-round-int-too-large eval ---
// Error: 2-33 the result is too large
#calc.round(int.max, digits: -1)

--- calc-round-int-negative-too-large eval ---
// Error: 2-34 the result is too large
#calc.round(-int.max, digits: -1)

--- calc-round-decimal-too-large eval ---
// Error: 2-66 the result is too large
#calc.round(decimal("79228162514264337593543950335"), digits: -1)

--- calc-round-decimal-negative-too-large eval ---
// Error: 2-67 the result is too large
#calc.round(decimal("-79228162514264337593543950335"), digits: -1)

--- calc-min-nothing eval ---
// Error: 2-12 expected at least one value
#calc.min()

--- calc-min-uncomparable eval ---
// Error: 14-18 cannot compare string and integer
#calc.min(1, "hi")

--- calc-max-uncomparable eval ---
// Error: 16-19 cannot compare 1pt with 1em
#calc.max(1em, 1pt)

--- calc-clamp-decimal-float eval ---
// Error: 2-37 cannot apply this operation to a decimal and a float
// Hint: 2-37 if loss of precision is acceptable, explicitly cast the decimal to a float with `float(value)`
#calc.clamp(decimal("10"), 5.5, 6.6)

--- calc-norm eval ---
#test(calc.norm(1, 2, -3, 0.5), calc.sqrt(14.25))
#test(calc.norm(3, 4), 5.0)
#test(calc.norm(3, 4), 5.0)
#test(calc.norm(), 0.0)
#test(calc.norm(p: 3, 1, -2), calc.pow(9, 1/3))
#test(calc.norm(p: calc.inf, 1, -2), 2.0)
#test(calc.norm(p: 309, 10), 10)
#test(calc.norm(p: 2, 0), 0)
#test(calc.norm(p: 100, 1210), 1210)

--- calc-norm-negative-p eval ---
// Error: 15-17 p must be greater than zero
#calc.norm(p: -1, 1)

--- calc-norm-expected-float eval ---
// Error: 12-15 expected float, found ratio
#calc.norm(10%)

--- calc-asinh eval ---
#let t(a, b) = assert(calc.abs(a - b) < 1e-6)
#t(calc.asinh(0), 0.0)
#t(calc.asinh(1), calc.ln(1 + calc.sqrt(2)))

--- calc-acosh eval ---
#let t(a, b) = assert(calc.abs(a - b) < 1e-6)
#t(calc.acosh(1), 0.0)
#t(calc.acosh(2), calc.ln(2 + calc.sqrt(3)))

--- calc-acosh-less-than-1 eval ---
// Error: 13-16 value must be greater than or equal to 1
#calc.acosh(0.5)

--- calc-atanh eval ---
#let t(a, b) = assert(calc.abs(a - b) < 1e-6)
#t(calc.atanh(0), 0.0)
#t(calc.atanh(0.5), 0.5 * calc.ln(3))

--- calc-atanh-less-than-negative-1 eval ---
// Error: 13-15 value must be between -1 and 1 (exclusive)
#calc.atanh(-2)

--- calc-atanh-greater-than-1 eval ---
// Error: 13-14 value must be between -1 and 1 (exclusive)
#calc.atanh(2)
//@@ file tests/suite/foundations_content.typ
--- content-at-default eval ---
// Test .at() default values for content.
#test(auto, [a].at("doesn't exist", default: auto))

--- content-field-syntax paged empty ---
// Test fields on elements.
#show list: it => {
  test(it.children.len(), 3)
}

- A
- B
- C

--- content-field-missing paged ---
// Error: 25-28 heading does not have field "fun"
#show heading: it => it.fun
= A

--- content-fields eval ---
// Test content fields method.
#test([a].fields(), (text: "a"))
#test([a *b*].fields(),  (children: ([a], [ ], strong[b])))

--- content-fields-mutable-invalid eval ---
#{
  let object = [hi]
  // Error: 3-9 cannot mutate fields on content
  object.property = "value"
}

--- content-field-materialized-table paged ---
// Ensure that fields from set rules are materialized into the element before
// a show rule runs.
#set table(columns: (10pt, auto))
#show table: it => it.columns
#table[A][B][C][D]

--- content-field-materialized-heading paged ---
// Test it again with a different element.
#set heading(numbering: "(I)")
#show heading: set text(size: 11pt, weight: "regular")
#show heading: it => it.numbering
= Heading

--- content-field-materialized-query paged ---
// Test it with query.
#set raw(lang: "rust")
#context query(<myraw>).first().lang
`raw` <myraw>

--- content-fields-complex paged ---
// Integrated test for content fields. The idea is to parse a normal looking
// equation and symbolically evaluate it with the given variable values.

#let compute(equation, ..vars) = {
  let vars = vars.named()
  let f(elem) = {
    let func = elem.func()
    if elem.has("text") {
      let text = elem.text
      if regex("^\\d+$") in text {
        int(text)
      } else if text in vars {
        int(vars.at(text))
      } else {
        panic("unknown math variable: " + text)
      }
    } else if func == math.attach {
      let value = f(elem.base)
      if elem.has("t") {
        value = calc.pow(value, f(elem.t))
      }
      value
    } else if elem.has("children") {
      elem
        .children
        .filter(v => v != [ ])
        .split($+$.body)
        .map(xs => xs.fold(1, (prod, v) => prod * f(v)))
        .fold(0, (sum, v) => sum + v)
    }
  }
  let result = f(equation.body)
  [With ]
  vars
    .pairs()
    .map(((name, value)) => $#symbol(name) = value$)
    .join(", ", last: " and ")
  [ we have:]
  $ equation = result $
}

#compute($x y + y^2$, x: 2, y: 3)
// This should generate the same output as:
// With $x = 2$ and $y = 3$ we have: $ x y + y^2 = 15 $

--- content-label-has-method paged ---
// Test whether the label is accessible through the `has` method.
#show heading: it => {
  assert(it.has("label"))
  it
}

= Hello, world! <my-label>

--- content-label-field-access paged ---
// Test whether the label is accessible through field syntax.
#show heading: it => {
  assert(str(it.label) == "my-label")
  it
}

= Hello, world! <my-label>

--- content-label-fields-method paged ---
// Test whether the label is accessible through the fields method.
#show heading: it => {
  assert("label" in it.fields())
  assert(str(it.fields().label) == "my-label")
  it
}

= Hello, world! <my-label>

--- content-fields-unset eval ---
// Error: 10-15 field "block" in raw is not known at this point
#raw("").block

--- content-fields-unset-no-default eval ---
// Error: 2-21 field "block" in raw is not known at this point and no default was specified
#raw("").at("block")

--- content-try-to-access-internal-field eval ---
// Error: 9-15 hide does not have field "hidden"
#hide[].hidden

--- content-spanned-field-equality eval ---
#assert.eq(heading(supplement: [A])[B], heading(supplement: [A])[B])
#assert.ne(heading(supplement: [A])[B], heading(supplement: [C])[B])
//@@ file tests/suite/foundations_context.typ
// Test context expressions.

--- context-body-atomic-in-markup eval ---
// Test that context body is parsed as atomic expression.
#let c = [#context "hello".]
#test(c.children.first().func(), (context none).func())
#test(c.children.last(), [.])

--- context-element-constructor-forbidden eval ---
// Test that manual construction is forbidden.
// Error: 2-25 cannot be constructed manually
#(context none).func()()

--- context-in-show-rule paged empty ---
// Test that show rule establishes context.
#set heading(numbering: "1.")
#show heading: it => test(
  counter(heading).get(),
  (intro: (1,), back: (2,)).at(str(it.label)),
)

= Introduction <intro>
= Background <back>

--- context-in-show-rule-query paged ---
// Test that show rule on non-locatable element allows `query`.
// Error: 18-47 Assertion failed: 2 != 3
#show emph: _ => test(query(heading).len(), 3)
#show strong: _ => test(query(heading).len(), 2)
= Introduction
= Background
*Hi* _there_

--- context-assign-to-captured-variable paged ---
// Test error when captured variable is assigned to.
#let i = 0
// Error: 11-12 variables from outside the context expression are read-only and cannot be modified
#context (i = 1)

--- context-delayed-warning paged empty ---
// Ensure that the warning that triggers in the first layout iteration is not
// surfaced since it goes away in the second one. Just like errors in show
// rules.
#show heading: none

= A <a>
#context {
  let n = query(<a>).len()
  let fonts = ("nope", "Roboto")
  set text(font: fonts.at(n))
}

--- context-body-is-closure paged ---
// Regression test since this used to be a hard crash.
#(context (a: none) => {})
//@@ file tests/suite/foundations_datetime.typ
--- datetime-constructor-empty eval ---
// Error: 2-12 at least one of date or time must be fully specified
// Hint: 2-12 add the `hour`, `minute`, and `second` arguments to get a valid time
// Hint: 2-12 add the `year`, `month`, and `day` arguments to get a valid date
#datetime()

--- datetime-constructor-time-invalid eval ---
// Error: 2-42 time is invalid
#datetime(hour: 25, minute: 0, second: 0)

--- datetime-constructor-date-invalid eval ---
// Error: 2-41 date is invalid
#datetime(year: 2000, month: 2, day: 30)

--- datetime-display eval ---
// Test displaying of dates.
#test(datetime(year: 2023, month: 4, day: 29).display(), "2023-04-29")
#test(datetime(year: 2023, month: 4, day: 29).display("[year]"), "2023")
#test(
  datetime(year: 2023, month: 4, day: 29)
    .display("[year repr:last_two]"),
  "23",
)
#test(
  datetime(year: 2023, month: 4, day: 29)
    .display("[year] [month repr:long] [day] [week_number] [weekday]"),
  "2023 April 29 17 Saturday",
)

// Test displaying of times
#test(datetime(hour: 14, minute: 26, second: 50).display(), "14:26:50")
#test(datetime(hour: 14, minute: 26, second: 50).display("[hour]"), "14")
#test(
  datetime(hour: 14, minute: 26, second: 50)
    .display("[hour repr:12 padding:none]"),
  "2",
)
#test(
  datetime(hour: 14, minute: 26, second: 50)
    .display("[hour], [minute], [second]"), "14, 26, 50",
)

// Test displaying of datetimes
#test(
  datetime(year: 2023, month: 4, day: 29, hour: 14, minute: 26, second: 50).display(),
  "2023-04-29 14:26:50",
)

// Test getting the year/month/day etc. of a datetime
#let d = datetime(year: 2023, month: 4, day: 29, hour: 14, minute: 26, second: 50)
#test(d.year(), 2023)
#test(d.month(), 4)
#test(d.weekday(), 6)
#test(d.day(), 29)
#test(d.hour(), 14)
#test(d.minute(), 26)
#test(d.second(), 50)

#let e = datetime(year: 2023, month: 4, day: 29)
#test(e.hour(), none)
#test(e.minute(), none)
#test(e.second(), none)

// Test today
#test(datetime.today().display(), "1970-01-01")
#test(datetime.today(offset: auto).display(), "1970-01-01")
#test(datetime.today(offset: 2).display(), "1970-01-01")
#test(datetime.today(offset: 14).display(), "1970-01-02")
#test(datetime.today(offset: -14).display(), "1969-12-31")
#test(datetime.today(offset: duration(hours: 5, minutes: 45)).display(), "1970-01-01")

--- datetime-ordinal eval ---
// Test date methods.
#test(datetime(day: 1, month: 1, year: 2000).ordinal(), 1);
#test(datetime(day: 1, month: 3, year: 2000).ordinal(), 31 + 29 + 1);
#test(datetime(day: 31, month: 12, year: 2000).ordinal(), 366);
#test(datetime(day: 1, month: 3, year: 2001).ordinal(), 31 + 28 + 1);
#test(datetime(day: 31, month: 12, year: 2001).ordinal(), 365);

--- datetime-incomplete-time-1 eval ---
// Error: 2-34 time is incomplete
// Hint: 2-34 add the `hour` argument to get a valid time
#datetime(minute: 14, second: 30)

--- datetime-incomplete-time-2 eval ---
// Error: 2-20 time is incomplete
// Hint: 2-20 add the `minute` and `second` arguments to get a valid time
#datetime(hour: 14)

--- datetime-incomplete-date-1 eval ---
// Error: 2-31 date is incomplete
// Hint: 2-31 add the `month` argument to get a valid date
#datetime(year: 2014, day: 30)

--- datetime-incomplete-date-2 eval ---
// Error: 2-20 date is incomplete
// Hint: 2-20 add the `year` and `day` arguments to get a valid date
#datetime(month: 5)

--- datetime-today-offset-invalid-1 eval ---
// Error: 2-28 unable to get the current date
#datetime.today(offset: 24)

--- datetime-today-offset-invalid-2 eval ---
// Error: 2-44 unable to get the current date
#datetime.today(offset: duration(days: -2))

--- datetime-display-missing-closing-bracket eval ---
// Error: 27-34 missing closing bracket for bracket at index 0
#datetime.today().display("[year")

--- datetime-display-invalid-component eval ---
// Error: 27-38 invalid component name 'nothing' at index 1
#datetime.today().display("[nothing]")

--- datetime-display-invalid-modifier eval ---
// Error: 27-50 invalid modifier 'wrong' at index 6
#datetime.today().display("[year wrong:last_two]")

--- datetime-display-expected-component eval ---
// Error: 27-33 expected component name at index 2
#datetime.today().display("  []")

--- datetime-display-insufficient-information eval ---
// Error: 2-36 failed to format datetime (insufficient information)
#datetime.today().display("[hour]")
//@@ file tests/suite/foundations_decimal.typ
--- decimal-constructor eval ---
#test(decimal(10), decimal("10.0"))
#test(decimal("-7654.321"), decimal("-7654.321"))
#test(decimal("\u{2212}7654.321"), decimal("-7654.321"))
#test(decimal({ 3.141592653 }), decimal("3.141592653000000012752934707"))
#test(decimal({ -3.141592653 }), decimal("-3.141592653000000012752934707"))
#test(decimal(decimal(3)), decimal("3.0"))
#test(decimal(true), decimal("1.0"))
#test(type(decimal(10)), decimal)

--- decimal-constructor-bad-type eval ---
// Error: 10-17 expected decimal, integer, boolean, float, or string, found type
#decimal(decimal)

--- decimal-constructor-bad-value eval ---
// Error: 10-17 invalid decimal: 1.2.3
#decimal("1.2.3")

--- decimal-constructor-float-literal eval ---
// Warning: 18-25 creating a decimal using imprecise float literal
// Hint: 18-25 use a string in the decimal constructor to avoid loss of precision: `decimal("1.32523")`
#let _ = decimal(1.32523)

--- decimal-constructor-float-inf eval ---
// Error: 10-19 float is not a valid decimal: float.inf
#decimal(float.inf)

--- decimal-constructor-float-negative-inf eval ---
// Error: 10-20 float is not a valid decimal: -float.inf
#decimal(-float.inf)

--- decimal-constructor-float-nan eval ---
// Error: 10-19 float is not a valid decimal: float.nan
#decimal(float.nan)

--- decimal-scale-is-observable eval ---
// Ensure equal decimals with different scales produce different strings.
#let f1(x) = str(x)
#let f2(x) = f1(x)
#test(f2(decimal("3.140")), "3.140")
#test(f2(decimal("3.14000")), "3.14000")

--- decimal-repr eval ---
// Test the `repr` function with decimals.
#test(repr(decimal("12.0")), "decimal(\"12.0\")")
#test(repr(decimal("3.14")), "decimal(\"3.14\")")
#test(repr(decimal("1234567890.0")), "decimal(\"1234567890.0\")")
#test(repr(decimal("0123456789.0")), "decimal(\"123456789.0\")")
#test(repr(decimal("0.0")), "decimal(\"0.0\")")
#test(repr(decimal("-0.0")), "decimal(\"0.0\")")
#test(repr(decimal("-1.0")), "decimal(\"-1.0\")")
#test(repr(decimal("-9876543210.0")), "decimal(\"-9876543210.0\")")
#test(repr(decimal("-0987654321.0")), "decimal(\"-987654321.0\")")
#test(repr(decimal("-3.14")), "decimal(\"-3.14\")")
#test(repr(decimal("-3.9191919191919191919191919195")), "decimal(\"-3.9191919191919191919191919195\")")
#test(repr(decimal("5.0000000000")), "decimal(\"5.0000000000\")")
#test(repr(decimal("4.0") - decimal("8.0")), "decimal(\"-4.0\")")

--- decimal-display paged ---
// Test decimals.
#set page(width: auto)
#decimal("12.0") \
#decimal("3.14") \
#decimal("1234567890.0") \
#decimal("0123456789.0") \
#decimal("0.0") \
#decimal("-0.0") \
#decimal("-1.0") \
#decimal("-9876543210.0") \
#decimal("-0987654321.0") \
#decimal("-3.14") \
#decimal("-3.9191919191919191919191919195") \
#decimal("5.0000000000") \
#(decimal("4.0") - decimal("8.0"))

--- decimal-display-round paged ---
// Display less digits.
#calc.round(decimal("-3.9191919191919191919191919195"), digits: 4) \
#calc.round(decimal("5.0000000000"), digits: 4)

--- decimal-expected-float-error eval ---
// Error: 11-25 expected integer, float, or angle, found decimal
// Hint: 11-25 if loss of precision is acceptable, explicitly cast the decimal to a float with `float(value)`
#calc.sin(decimal("1.1"))

--- decimal-expected-integer-error eval ---
// Error: 11-25 expected integer, found decimal
#calc.odd(decimal("1.1"))
//@@ file tests/suite/foundations_dict.typ
// Test dictionaries.

// Tests for method calls on dictionaries are in `../scripting/methods.typ` as
// `field-call-dict-*`.

--- dict-basic-syntax paged ---

// Empty
#(:)

// Two pairs and string key.
#let dict = (normal: 1, "spacy key": 2)
#dict

#test(dict.normal, 1)
#test(dict.at("spacy key"), 2)

--- dict-fields eval ---
// Test field on dictionary.
#let dict = (nothing: "ness", hello: "world")
#test(dict.nothing, "ness")
#{
  let world = dict
    .hello

  test(world, "world")
}

--- dict-missing-field eval ---
// Error: 6-13 dictionary does not contain key "invalid"
#(:).invalid

--- dict-bad-key eval ---
// Error: 3-7 expected string, found boolean
// Error: 16-18 expected string, found integer
#(true: false, 42: 3)

--- dict-duplicate-key eval ---
// Error: 24-29 duplicate key: first
#(first: 1, second: 2, first: 3)

--- dict-duplicate-key-stringy eval ---
// Error: 17-20 duplicate key: a
#(a: 1, "b": 2, "a": 3)

--- dict-bad-expression eval ---
// Simple expression after already being identified as a dictionary.
// Error: 9-10 expected named or keyed pair, found identifier
#(a: 1, b)

--- dict-leading-colon eval ---
// Identified as dictionary due to initial colon.
// The boolean key is allowed for now since it will only cause an error at the evaluation stage.
// Error: 4-5 expected named or keyed pair, found integer
// Error: 17 expected expression
#(:1 b:"", true:)

--- spread-into-dict eval ---
#{
  let x = (a: 1)
  let y = (b: 2)
  let z = (a: 3)
  test((:..x, ..y, ..z), (a: 3, b: 2))
  test((..(a: 1), b: 2), (a: 1, b: 2))
}

--- spread-dicts-in-array-diagnostic eval ---
#{
  let x = (a: 1)
  let y = (b: 2)
  // Error: 4-7 cannot spread dictionary into array
  // Hint: 4-7 add a colon to create a dictionary instead: `(: ..x,..y)`
  (..x,..y)
}

--- spread-single-dict-in-array-diagnostic eval ---
#{
  let x = (a: 1)
  // Error: 4-7 cannot spread dictionary into array
  // Hint: 4-7 add a colon to create a dictionary instead: `(: ..x)`
  (..x)
}

--- spread-dict-and-array-in-array-diagnostic eval ---
// Here we should not emit a hint to use `(: <...>)` syntax
// since the developer's intent is unclear
#{
  let x = (a: 1)
  let y = (2,3)
  // Error: 4-7 cannot spread dictionary into array
  (..x,..y)
}

--- spread-dict-and-single-item eval ---
// Here we should not emit a hint to use `(: <...>)` syntax
// since the developer's intent is unclear
#{
  let x = (a: 1)
  // Error: 4-7 cannot spread dictionary into array
  (..x,"item")
}

--- spread-none-and-dict eval ---
#{
  let x = (a: 1)
  // Error: 11-20 cannot spread dictionary into array
  // Hint: 11-20 add a colon to create a dictionary instead: `(: ..none,..(one:1))`
  (..none,..(one:1))
}

--- spread-array-into-dict eval ---
// Error: 3-11 cannot spread array into dictionary
#(..(1, 2), a: 1)

--- dict-at-lvalue eval ---
// Test lvalue and rvalue access.
#{
  let dict = (a: 1, "b b": 1)
  dict.at("b b") += 1
  dict.state = (ok: true, err: false)
  test(dict, (a: 1, "b b": 2, state: (ok: true, err: false)))
  test(dict.state.ok, true)
  dict.at("state").ok = false
  test(dict.state.ok, false)
  test(dict.state.err, false)
}

--- dict-at-call eval ---
// Test calling a function in a dictionary via `.at()`.
#let dict = (func: x => x + 1)
#test(dict.at("func")(0), 1)

--- dict-at-missing-key eval ---
// Test rvalue missing key.
#{
  let dict = (a: 1, b: 2)
  // Error: 11-23 dictionary does not contain key "c" and no default value was specified
  let x = dict.at("c")
}

--- dict-at-default eval ---
// Test default value.
#test((a: 1, b: 2).at("b", default: 3), 2)
#test((a: 1, b: 2).at("c", default: 3), 3)

--- dict-insert eval ---
// Test insert.
#{
  let dict = (a: 1, b: 2)
  dict.insert("b", 3)
  test(dict, (a: 1, b: 3))
  dict.insert("c", 5)
  test(dict, (a: 1, b: 3, c: 5))
}

--- dict-remove-with-default eval ---
// Test remove with default value.
#{
  let dict = (a: 1, b: 2)
  test(dict.remove("b", default: 3), 2)
}

#{
  let dict = (a: 1, b: 2)
  test(dict.remove("c", default: 3), 3)
}

--- dict-missing-lvalue eval ---
// Missing lvalue is not automatically none-initialized.
#{
  let dict = (:)
  // Error: 3-9 dictionary does not contain key "b"
  // Hint: 3-9 use `insert` to add or update values
  dict.b += 1
}

--- dict-basic-methods eval ---
// Test dictionary methods.
#let dict = (a: 3, c: 2, b: 1)
#test("c" in dict, true)
#test(dict.len(), 3)
#test(dict.values(), (3, 2, 1))
#test(dict.pairs().map(p => p.first() + str(p.last())).join(), "a3c2b1")

#test(dict.remove("c"), 2)
#test("c" in dict, false)
#test(dict, (a: 3, b: 1))

--- dict-from-module eval ---
// Test dictionary constructor
#test(type(dictionary(sys).at("version")), version)
#test(dictionary(sys).at("no-crash", default: none), none)

--- dict-from-module-feature-gated-item eval features() ---
// Error: 20-25 dictionary does not contain key "gated"
#dictionary(check).gated

--- dict-from-module-deprecation-ignored eval ---
#dictionary(check).deprecated

--- dict-remove-order eval ---
// Test that removal keeps order.
#let dict = (a: 1, b: 2, c: 3, d: 4)
#test(dict.remove("b"), 2)
#test(dict.keys(), ("a", "c", "d"))

--- dict-insert-order eval ---
#let dict = (a: 1, b: 2)
#let rhs = (c: 3, a: 4)

// Add
#test((dict + rhs).keys(), ("a", "b", "c"))

// Join
#test({ dict; rhs }.keys(), ("a", "b", "c"))

// Spread
#test((:..dict, ..rhs).keys(), ("a", "b", "c"))

// Insert
#{
  for (k, v) in rhs {
    dict.insert(k, v)
  }
  test(dict.keys(), ("a", "b", "c"))
}

// Assign
#{
  dict.a = 5
  dict.d = 6
  test(dict.keys(), ("a", "b", "c", "d"))
}

--- dict-temporary-lvalue eval ---
// Error: 3-15 cannot mutate a temporary value
#((key: "val").other = "some")

--- dict-method-typo eval ---
// Error: 2-25 type dictionary has no method `rmeove`
#(typo: "rmeove").rmeove("typo")

--- dict-dynamic-duplicate-key eval ---
#let a = "hello"
#let b = "world"
#let c = "value"
#let d = "conflict"

#test(((a): b), ("hello": "world"))
#test(((a): 1, (a): 2), ("hello": 2))
#test((hello: 1, (a): 2), ("hello": 2))
#test((a + b: c, (a + b): d, (a): "value2", a: "value3"), ("helloworld": "conflict", "hello": "value2", "a": "value3"))

--- dict-filter eval ---
// Test the `filter` method.
#test((:).filter(calc.even), (:))
#test((a: 0, b: 1, c: 2).filter(v => v != 0), (b: 1, c: 2))
#test((a: 0, b: 1, c: 2).filter(calc.even), (a: 0, c: 2))

--- dict-filter-error eval ---
// Test that errors in the predicate are reported properly.
// Error: 23-28 cannot subtract integer from string
#(a: "a").filter(v => v - 2)

--- dict-map eval ---
// Test the `map` method.
#test(().map(x => x * 2), ())
#test((a: 2, b: 3).map(x => x * 2), (a: 4, b: 6))

--- dict-map-error eval ---
// Test that errors in the function are reported properly.
// Error: 20-25 cannot subtract integer from string
#(a: "a").map(v => v - 2)

--- issue-1338-dictionary-underscore eval ---
#let foo = "foo"
#let bar = "bar"
// Error: 8-9 expected expression, found underscore
// Error: 16-17 expected expression, found underscore
#(foo: _, bar: _)

--- issue-1342-dictionary-bare-expressions eval ---
// Error: 5-8 expected named or keyed pair, found identifier
// Error: 10-13 expected named or keyed pair, found identifier
#(: foo, bar)

--- issue-3154-dict-at-not-contained eval ---
#{
  let dict = (a: 1)
  // Error: 3-15 dictionary does not contain key "b" and no default value was specified
  dict.at("b")
}

--- issue-3154-dict-at-missing-default eval ---
#{
  let dict = (a: 1)
  test(dict.at("b", default: 0), 0)
}

--- issue-3154-dict-at-missing-mutable eval ---
#{
  let dict = (a: 1)
  // Error: 3-15 dictionary does not contain key "b"
  // Hint: 3-15 use `insert` to add or update values
  dict.at("b") = 9
}

--- issue-3154-dict-at-missing-mutable-default eval ---
#{
  let dict = (a: 1)
  // Error: 3-27 dictionary does not contain key "b"
  // Hint: 3-27 use `insert` to add or update values
  dict.at("b", default: 0) = 9
}

--- issue-3154-dict-syntax-missing eval ---
#{
  let dict = (a: 1)
  // Error: 8-9 dictionary does not contain key "b"
  dict.b
}

--- issue-3154-dict-syntax-missing-mutable eval ---
#{
  let dict = (a: 1)
  dict.b = 9
  test(dict, (a: 1, b: 9))
}

--- issue-3154-dict-syntax-missing-add-assign eval ---
#{
  let dict = (a: 1)
  // Error: 3-9 dictionary does not contain key "b"
  // Hint: 3-9 use `insert` to add or update values
  dict.b += 9
}

--- issue-3232-dict-unexpected-keys-sides eval ---
// Confusing "expected relative length or dictionary, found dictionary"
// Error: 16-58 unexpected keys "unexpected" and "unexpected-too"
#block(outset: (unexpected: 0.5em, unexpected-too: 0.2em), [Hi])

--- issue-3232-dict-unexpected-keys-corners eval ---
// Error: 14-56 unexpected keys "unexpected" and "unexpected-too"
#box(radius: (unexpected: 0.5em, unexpected-too: 0.5em), [Hi])

--- issue-3232-dict-unexpected-key-sides eval ---
// Error: 16-49 unexpected key "unexpected", valid keys are "left", "top", "right", "bottom", "x", "y", and "rest"
#block(outset: (unexpected: 0.2em, right: 0.5em), [Hi]) // The 1st key is unexpected

--- issue-3232-dict-unexpected-key-corners eval ---
// Error: 14-50 unexpected key "unexpected", valid keys are "top-left", "top-right", "bottom-right", "bottom-left", "left", "top", "right", "bottom", and "rest"
#box(radius: (top-left: 0.5em, unexpected: 0.5em), [Hi]) // The 2nd key is unexpected

--- issue-3232-dict-empty paged ---
#block(outset: (:), [Hi]) // Ok
#box(radius: (:), [Hi]) // Ok
//@@ file tests/suite/foundations_duration.typ
// Test durations.

--- duration-negate eval ---
// Test negating durations.
#test(-duration(hours: 2), duration(hours: -2))

--- duration-add-and-subtract eval ---
// Test adding and subtracting durations.
#test(duration(weeks: 1, hours: 1), duration(weeks: 1) + duration(hours: 1))
#test(duration(weeks: 1, hours: -1), duration(weeks: 1) - duration(hours: 1))
#test(duration(days: 6, hours: 23), duration(weeks: 1) - duration(hours: 1))

--- duration-add-and-subtract-dates eval ---
// Test adding and subtracting durations and dates.
#let d = datetime(day: 1, month: 1, year: 2000)
#let d2 = datetime(day: 1, month: 2, year: 2000)
#test(d + duration(weeks: 2), datetime(day: 15, month: 1, year: 2000))
#test(d + duration(days: 3), datetime(day: 4, month: 1, year: 2000))
#test(d + duration(weeks: 1, days: 3), datetime(day: 11, month: 1, year: 2000))
#test(d2 + duration(days: -1), datetime(day: 31, month: 1, year: 2000))
#test(d2 + duration(days: -3), datetime(day: 29, month: 1, year: 2000))
#test(d2 + duration(weeks: -1), datetime(day: 25, month: 1, year: 2000))
#test(d + duration(days: -1), datetime(day: 31, month: 12, year: 1999))
#test(d + duration(weeks: 1, days: -7), datetime(day: 1, month: 1, year: 2000))
#test(d2 - duration(days: 1), datetime(day: 31, month: 1, year: 2000))
#test(d2 - duration(days: 3), datetime(day: 29, month: 1, year: 2000))
#test(d2 - duration(weeks: 1), datetime(day: 25, month: 1, year: 2000))
#test(d - duration(days: 1), datetime(day: 31, month: 12, year: 1999))
#test(datetime(day: 31, month: 1, year: 2000) + duration(days: 1), d2)
#test(
  datetime(day: 31, month: 12, year: 2000) + duration(days: 1),
  datetime(day: 1, month: 1, year: 2001),
)

--- duration-add-and-subtract-times eval ---
// Test adding and subtracting durations and times.
#let a = datetime(hour: 12, minute: 0, second: 0)
#test(a + duration(hours: 1, minutes: -60), datetime(hour: 12, minute: 0, second: 0))
#test(a + duration(hours: 2), datetime(hour: 14, minute: 0, second: 0))
#test(a + duration(minutes: 10), datetime(hour: 12, minute: 10, second: 0))
#test(a + duration(seconds: 30), datetime(hour: 12, minute: 0, second: 30))
#test(a + duration(hours: -2), datetime(hour: 10, minute: 0, second: 0))
#test(a - duration(hours: 2), datetime(hour: 10, minute: 0, second: 0))
#test(a + duration(minutes: -10), datetime(hour: 11, minute: 50, second: 0))
#test(a - duration(minutes: 10), datetime(hour: 11, minute: 50, second: 0))
#test(a + duration(seconds: -30), datetime(hour: 11, minute: 59, second: 30))
#test(a - duration(seconds: 30), datetime(hour: 11, minute: 59, second: 30))
#test(
  a + duration(hours: 1, minutes: 13, seconds: 13),
  datetime(hour: 13, minute: 13, second: 13),
)

--- duration-add-and-subtract-datetimes eval ---
// Test adding and subtracting durations and datetimes.
#test(
  datetime(day: 1, month: 1, year: 2000, hour: 12, minute: 0, second: 0)
    + duration(weeks: 1, days: 3, hours: -13, minutes: 10, seconds: -10 ),
  datetime(day: 10, month: 1, year: 2000, hour: 23, minute: 9, second: 50),
)
#test(
  datetime(day: 1, month: 1, year: 2000, hour: 12, minute: 0, second: 0)
    + duration(weeks: 1, days: 3, minutes: 10)
    - duration(hours: 13, seconds: 10),
  datetime(day: 10, month: 1, year: 2000, hour: 23, minute: 9, second: 50),
)

--- duration-from-date-subtraction eval ---
// Test subtracting dates.
#let a = datetime(hour: 12, minute: 0, second: 0)
#let b = datetime(day: 1, month: 1, year: 2000)
#test(datetime(hour: 14, minute: 0, second: 0) - a, duration(hours: 2))
#test(datetime(hour: 14, minute: 0, second: 0) - a, duration(minutes: 120))
#test(datetime(hour: 13, minute: 0, second: 0) - a, duration(seconds: 3600))
#test(datetime(day: 1, month: 2, year: 2000) - b, duration(days: 31))
#test(datetime(day: 15, month: 1, year: 2000) - b, duration(weeks: 2))

--- issue-7843-date-duration-precision eval ---
#test(
  datetime(year: 2030, month: 5, day: 25,            hour:  6, minute:  30, second:  19),
  datetime(year: 2030, month: 5, day: 25) + duration(hours: 6, minutes: 30, seconds: 19),
)
#test(
  datetime(year: 2030, month: 5, day: 25),
  datetime(year: 2030, month: 5, day: 26) - duration(hours: 24),
)

#let some-day = datetime(year: 2000, month: 6, day: 5)
#let two-hours = duration(hours: 2)
#test(some-day + 100 * two-hours, range(100).fold(some-day, (d, _) => d + two-hours))
#test(some-day - 100 * two-hours, range(100).fold(some-day, (d, _) => d - two-hours))

--- duration-multiply-with-number eval ---
// Test multiplying and dividing durations with numbers.
#test(duration(minutes: 10) * 6, duration(hours: 1))
#test(duration(minutes: 10) * 2, duration(minutes: 20))
#test(duration(minutes: 10) * 2.5, duration(minutes: 25))
#test(duration(minutes: 10) / 2, duration(minutes: 5))
#test(duration(minutes: 10) / 2.5, duration(minutes: 4))

--- duration-divide eval ---
// Test dividing durations with durations
#test(duration(minutes: 20) / duration(hours: 1), 1 / 3)
#test(duration(minutes: 20) / duration(minutes: 10), 2)
#test(duration(minutes: 20) / duration(minutes: 8), 2.5)

--- duration-compare eval ---
// Test comparing durations
#test(duration(minutes: 20) > duration(minutes: 10), true)
#test(duration(minutes: 20) >= duration(minutes: 10), true)
#test(duration(minutes: 10) < duration(minutes: 20), true)
#test(duration(minutes: 10) <= duration(minutes: 20), true)
#test(duration(minutes: 10) == duration(minutes: 10), true)
#test(duration(minutes: 10) != duration(minutes: 20), true)
#test(duration(minutes: 10) <= duration(minutes: 10), true)
#test(duration(minutes: 10) >= duration(minutes: 10), true)
#test(duration(minutes: 20) < duration(minutes: 10), false)
#test(duration(minutes: 20) <= duration(minutes: 10), false)
#test(duration(minutes: 20) == duration(minutes: 10), false)
//@@ file tests/suite/foundations_eval.typ
--- eval eval ---
// Test the eval function.
#test(eval("1 + 2"), 3)
#test(eval("1 + x", scope: (x: 3)), 4)
#test(eval("let x = x + 1; x + 1", scope: (x: 1)), 3)

--- eval-mode paged ---
// Test evaluation in other modes.
#eval("[_Hello" + " World!_]") \
#eval("_Hello" + " World!_", mode: "markup") \
#eval("RR_1^NN", mode: "math", scope: (RR: math.NN, NN: math.RR))

--- eval-syntax-error-1 eval ---
// Error: 7-12 expected pattern
// Hint: 7-12 at index `3`
#eval("let")

--- eval-in-show-rule paged ---
#show raw: it => text(font: "PT Sans", eval("[" + it.text + "]"))

Interacting
```
#set text(blue)
Blue #move(dy: -0.15em)[🌊]
```

--- eval-runtime-error eval ---
// Error: 7-17 cannot continue outside of loop
#eval("continue")

--- eval-syntax-error-2 eval ---
// Error: 7-12 expected semicolon or line break
// Hint: 7-12 at index `1`
#eval("1 2")

--- eval-path-resolve paged ---
// Test absolute path.
#eval("image(\"/assets/images/tiger.jpg\", width: 50%)")

--- eval-path-resolve-in-show-rule paged ---
#show raw: it => eval(it.text, mode: "markup")

```
#show emph: image("/assets/images/tiger.jpg", width: 50%)
_Tiger!_
```

--- eval-path-resolve-relative eval ---
// Test relative path.
#test(eval(`"HELLO" in read("./eval.typ")`.text), true)

--- issue-2055-math-eval paged ---
// Evaluating a math expr should renders the same as an equation
#eval(mode: "math", "f(a) = cases(a + b\, space space x >= 3,a + b\, space space x = 5)")

$f(a) = cases(a + b\, space space x >= 3,a + b\, space space x = 5)$

--- issue-6067-eval-warnings paged empty ---
// Test that eval shows warnings from the executed code.
// Warning: 7-11 no text within stars
// Hint: 7-11 using multiple consecutive stars (e.g. **) has no additional effect
// Hint: 7-11 from index `0` to `2`
#eval("**", mode: "markup")
//@@ file tests/suite/foundations_float.typ
--- float-constructor eval ---
#test(float(10), 10.0)
#test(float(50% * 30%), 0.15)
#test(float("31.4e-1"), 3.14)
#test(float("31.4e\u{2212}1"), 3.14)
#test(float("3.1415"), 3.1415)
#test(float("-7654.321"), -7654.321)
#test(float("\u{2212}7654.321"), -7654.321)
#test(float(decimal("4.89")), 4.89)
#test(float(decimal("3.1234567891234567891234567891")), 3.123456789123457)
#test(float(decimal("79228162514264337593543950335")), 79228162514264340000000000000.0)
#test(float(decimal("-79228162514264337593543950335")), -79228162514264340000000000000.0)
#test(type(float(10)), float)

--- float-constructor-bad-type eval ---
// Error: 8-13 expected float, boolean, integer, decimal, ratio, or string, found type
#float(float)

--- float-constructor-bad-value eval ---
// Error: 8-15 invalid float: 1.2.3
#float("1.2.3")

--- float-is-nan eval ---
// Test float `is-nan()`.
#test(float(float.nan).is-nan(), true)
#test(float(10).is-nan(), false)
#test(float(calc.inf).is-nan(), false)
#test(float(-calc.inf).is-nan(), false)

--- float-is-infinite eval ---
// Test float `is-infinite()`.
#test(float(calc.inf).is-infinite(), true)
#test(float(-calc.inf).is-infinite(), true)
#test(float(10).is-infinite(), false)
#test(float(-10).is-infinite(), false)
#test(float(float.nan).is-infinite(), false)

--- float-signum eval ---
// Test float `signum()`
#test(float(0.0).signum(), 1.0)
#test(float(1.0).signum(), 1.0)
#test(float(-1.0).signum(), -1.0)
#test(float(10.0).signum(), 1.0)
#test(float(-10.0).signum(), -1.0)
#test(float(calc.inf).signum(), 1.0)
#test(float(-calc.inf).signum(), -1.0)
#test(float(float.nan).signum().is-nan(), true)

--- float-from-and-to-bytes eval ---
// Test float `from-bytes()` and `to-bytes()`.
#test(float.from-bytes(bytes((0, 0, 0, 0, 0, 0, 240, 63))), 1.0)
#test(float.from-bytes(bytes((63, 240, 0, 0, 0, 0, 0, 0)), endian: "big"), 1.0)
#test(1.0.to-bytes(), bytes((0, 0, 0, 0, 0, 0, 240, 63)))
#test(1.0.to-bytes(endian: "big"), bytes((63, 240, 0, 0, 0, 0, 0, 0)))

#test(float.from-bytes(bytes((0, 0, 32, 64))), 2.5)
#test(float.from-bytes(bytes((64, 32, 0, 0)), endian: "big"), 2.5)
#test(2.5.to-bytes(size: 4), bytes((0, 0, 32, 64)))
#test(2.5.to-bytes(size: 4, endian: "big"), bytes((64, 32, 0, 0)))

--- float-from-bytes-bad-length eval ---
// Error: 2-54 bytes must have a length of 4 or 8
#float.from-bytes(bytes((0, 0, 0, 0, 0, 0, 0, 1, 0)))

--- float-repr eval ---
// Test the `repr` function with floats.
#test(repr(12.0), "12.0")
#test(repr(3.14), "3.14")
#test(repr(1234567890.0), "1234567890.0")
#test(repr(0123456789.0), "123456789.0")
#test(repr(0.0), "0.0")
#test(repr(-0.0), "-0.0")
#test(repr(-1.0), "-1.0")
#test(repr(-9876543210.0), "-9876543210.0")
#test(repr(-0987654321.0), "-987654321.0")
#test(repr(-3.14), "-3.14")
#test(repr(4.0 - 8.0), "-4.0")
#test(repr(float.inf), "float.inf")
#test(repr(-float.inf), "-float.inf")
#test(repr(float.nan), "float.nan")

--- float-display paged ---
// Test floats.
#12.0 \
#3.14 \
#1234567890.0 \
#0123456789.0 \
#0.0 \
#(-0.0) \
#(-1.0) \
#(-9876543210.0) \
#(-0987654321.0) \
#(-3.14) \
#(4.0 - 8.0) \
#float.inf \
#(-float.inf) \
#float.nan

--- float-syntax-edge-cases eval ---
// Test float syntax edge cases and which spans of text are highlighted. Valid
// items are those not annotated with an error comment since syntax is handled
// at parse time.

#123.456e+789
#000.000E-000
#.9e0
#123.E // this is a field access, so is fine syntactically
#0.e
#1.E+020
// Error: 2-10 invalid floating point number: `123.456e`
#123.456e
// Error: 2-11 invalid floating point number: `123.456e+`
#123.456e+
// Error: 2-6 invalid floating point number: `.1E-`
#.1E-
// Error: 2-4 invalid floating point number: `0e`
#0e
//@@ file tests/suite/foundations_int.typ
--- int-base-alternative eval ---
// Test numbers with alternative bases.
#test(0x10, 16)
#test(0b1101, 13)
#test(0xA + 0xa, 0x14)

--- int-base-binary-invalid eval ---
// Error: 2-7 integer contains digits that are not valid for a binary number
// Hint: 2-7 binary numbers only allow digits 0-1
// Hint: 5-7 the digits `2` and `3` are invalid
#0b123

--- int-base-octal-invalid eval ---
// Error: 2-11 integer contains digits that are not valid for an octal number
// Hint: 2-11 octal numbers only allow digits 0-7
// Hint: 7-9 the digits `8` and `9` are invalid
#0o1078970

--- int-base-hex-invalid eval ---
// Error: 2-9 integer contains digits that are not valid for a hexadecimal number
// Hint: 2-9 hexadecimal numbers only allow digits 0-9, a-f, A-F
// Hint: 7-8 the digit `z` is invalid
#0x123z4

--- int-base-binary-invalid-long eval ---
// Error: 2-14 integer contains digits that are not valid for a binary number
// Hint: 2-14 binary numbers only allow digits 0-1
// Hint: 6-14 the digits `2`, `3`, `4`, `5`, `6`, `7`, `8`, and `9` are invalid
#0b0123456789

--- int-base-octal-invalid-long eval ---
// Error: 2-14 integer contains digits that are not valid for an octal number
// Hint: 2-14 octal numbers only allow digits 0-7
// Hint: 12-14 the digits `8` and `9` are invalid
#0o0123456789

--- int-base-hex-invalid-long eval ---
// Error: 2-56 integer contains digits that are not valid for a hexadecimal number
// Hint: 2-56 hexadecimal numbers only allow digits 0-9, a-f, A-F
// Hint: 10-56 the digits `g`, `h`, `i`, `j`, `k`, `l`, `m`, `n`, `o`, `p`, `q`, `r`, `s`, `t`, `u`, `v`, `w`, `y`, `z`, `G`, `H`, `I`, `J`, `K`, `L`, `M`, `N`, `O`, `P`, `Q`, `R`, `S`, `T`, `U`, `V`, `W`, `X`, `Y`, and `Z` are invalid
#0xabcdefghijklmnopqrstuvwkyzABCDEFGHIJKLMNOPQRSTUVWXYZ

--- int-base-hex-invalid-non-ascii eval ---
// Error: 9 expected comma
#(0xabcdéf)

--- int-base-empty eval ---
// Error: 2-4 expected a hexadecimal number
#0x
// Error: 2-4 expected an octal number
#0o
// Error: 2-4 expected a binary number
#0b

--- int-max eval ---
#test(int.max, 9223372036854775807)
#test(int.max, 0x7FFFFFFFFFFFFFFF)

--- int-min eval ---
#test(int.min, -1 - int.max)
#test(int.min, int("-9223372036854775808"))

--- int-bounds-max-overflow eval ---
// Error: 3-14 value is too large
#(int.max + 1)

--- int-bounds-min-underflow eval ---
// TODO: Change this to "too small".
// Error: 3-14 value is too large
#(int.min - 1)

--- int-bounds-max-plus-one eval ---
// Error: 2-21 integer value is too large
// Hint: 2-21 value does not fit into a signed 64-bit integer
// Hint: 2-21 a floating point number could approximately represent this value
// Hint: 2-21 you can use a floating point number by appending a dot: `9223372036854775808.`
#9223372036854775808

--- int-bounds-max-plus-one-hex eval ---
// Error: 2-20 integer value is too large
// Hint: 2-20 value does not fit into a signed 64-bit integer
#0x8000000000000000

--- int-bounds-max-plus-two eval ---
// Error: 2-21 integer value is too large
// Hint: 2-21 value does not fit into a signed 64-bit integer
// Hint: 2-21 a floating point number could approximately represent this value
// Hint: 2-21 you can use a floating point number by appending a dot: `9223372036854775809.`
#9223372036854775809

--- int-bounds-max-u64 eval ---
// Error: 2-22 integer value is too large
// Hint: 2-22 value does not fit into a signed 64-bit integer
// Hint: 2-22 a floating point number could approximately represent this value
// Hint: 2-22 you can use a floating point number by appending a dot: `18446744073709551615.`
#18446744073709551615

--- int-bounds-manual-min-no-space eval ---
// Error: 3-23 cannot write minimum integer manually
// Hint: 3-23 Typst integers are always initially positive
// Hint: 3-23 2^63 does not fit into a signed 64-bit integer
// Hint: 3-23 try writing `int.min`
#(-9223372036854775808)

--- int-bounds-manual-min-space eval ---
// Error: 3-24 cannot write minimum integer manually
// Hint: 3-24 Typst integers are always initially positive
// Hint: 3-24 2^63 does not fit into a signed 64-bit integer
// Hint: 3-24 try writing `int.min`
#(- 9223372036854775808)

--- int-bounds-manual-min-newline eval ---
// Error: 1:3-2:23 cannot write minimum integer manually
// Hint: 1:3-2:23 Typst integers are always initially positive
// Hint: 1:3-2:23 2^63 does not fit into a signed 64-bit integer
// Hint: 1:3-2:23 try writing `int.min`
#(-
   9223372036854775808)

--- int-bounds-manual-min-hex eval ---
// Error: 3-22 cannot write minimum integer manually
// Hint: 3-22 Typst integers are always initially positive
// Hint: 3-22 2^63 does not fit into a signed 64-bit integer
// Hint: 3-22 try writing `int.min`
#(-0x8000000000000000)

--- int-bounds-min-minus-one eval ---
// Error: 3-23 integer value is too small
// Hint: 3-23 value does not fit into a signed 64-bit integer
// Hint: 3-23 a floating point number could approximately represent this value
// Hint: 3-23 you can use a floating point number by appending a dot: `9223372036854775809.`
#(-9223372036854775809)

--- int-bounds-invalid-no-syntax-error eval ---
// Some integer syntax errors happen in the AST, so we won't actually error if
// the int isn't evaluated.
#let _ = () => 9223372036854775808
#let _ = () => -9223372036854775808
#let _ = () => 0b123

--- int-constructor eval ---
// Test conversion to numbers.
#test(int(false), 0)
#test(int(true), 1)
#test(int(10), 10)
#test(int("0"), 0)
#test(int("+150"), 150)
#test(int("-834"), -834)
#test(int("beef", base: 16), 48879)
#test(int("-cAfFe", base: 16), -831486)
#test(int("10", base: 2), 2)
#test(int("644", base: 8), 420)
#test(int("\u{2212}79"), -79)
#test(int("9223372036854775807"), int.max)
#test(int("-9223372036854775808"), int.min)
#test(int("7FFFFFFFFFFFFFFF", base: 16), int.max)
#test(int("-8000000000000000", base: 16), int.min)
#test(int(10 / 3), 3)
#test(int(-58.34), -58)
#test(int(decimal("92492.193848921")), 92492)
#test(int(decimal("-224.342211")), -224)

--- int-constructor-bad-type eval ---
// Error: 6-10 expected integer, boolean, float, decimal, or string, found length
#int(10pt)

--- int-constructor-str-empty eval ---
// Error: 6-8 string must not be empty
#int("")

--- int-constructor-str-empty-based eval ---
// Error: 6-8 string must not be empty
#int("", base: 16)

--- int-constructor-bad-value eval ---
// Error: 6-12 string contains invalid digits
#int("nope")

--- int-constructor-bad-value-based eval ---
// Error: 6-11 string contains invalid digits for a base 3 integer
#int("123", base: 3)

--- int-constructor-base-with-non-string eval ---
// Error: 16-18 base is only supported for strings
#int(40, base: 16)

--- int-constructor-str-small-base eval ---
// Error: 17-18 base must be between 2 and 36
#int("0", base: 1)

--- int-constructor-str-large-base eval ---
// Error: 17-19 base must be between 2 and 36
#int("0", base: 42)

--- int-constructor-str-too-large eval ---
// Error: 6-27 integer value is too large
// Hint: 6-27 value does not fit into a signed 64-bit integer
// Hint: 6-27 try using a floating point number
#int("9223372036854775808")

--- int-constructor-str-too-small eval ---
// Error: 6-28 integer value is too small
// Hint: 6-28 value does not fit into a signed 64-bit integer
// Hint: 6-28 try using a floating point number
#int("-9223372036854775809")

--- int-constructor-float-too-large eval ---
// Error: 6-27 number too large
#int(9223372036854775809.5)

--- int-constructor-float-too-large-min eval ---
// Error: 6-28 number too large
#int(-9223372036854775809.5)

--- int-constructor-decimal-too-large eval ---
// Error: 6-38 number too large
#int(decimal("9223372036854775809.5"))

--- int-constructor-decimal-too-large-min eval ---
// Error: 6-39 number too large
#int(decimal("-9223372036854775809.5"))

--- int-signum eval ---
// Test int `signum()`
#test(int(0).signum(), 0)
#test(int(1.0).signum(), 1)
#test(int(-1.0).signum(), -1)
#test(int(10.0).signum(), 1)
#test(int(-10.0).signum(), -1)

--- int-from-and-to-bytes eval ---
// Test `int.from-bytes` and `int.to-bytes`.
#test(int.from-bytes(bytes(())), 0)
#test(int.from-bytes(bytes((1, 0, 0, 0, 0, 0, 0, 0)), endian: "little", signed: true), 1)
#test(int.from-bytes(bytes((1, 0, 0, 0, 0, 0, 0, 0)), endian: "big", signed: true), 72057594037927936)
#test(int.from-bytes(bytes((1, 0, 0, 0, 0, 0, 0, 0)), endian: "little", signed: false), 1)
#test(int.from-bytes(bytes((255,)), endian: "big", signed: true), -1)
#test(int.from-bytes(bytes((255,)), endian: "big", signed: false), 255)
#test(int.from-bytes((-1000).to-bytes(endian: "big", size: 5), endian: "big", signed: true), -1000)
#test(int.from-bytes((-1000).to-bytes(endian: "little", size: 5), endian: "little", signed: true), -1000)
#test(int.from-bytes(1000.to-bytes(endian: "big", size: 5), endian: "big", signed: true), 1000)
#test(int.from-bytes(1000.to-bytes(endian: "little", size: 5), endian: "little", signed: true), 1000)
#test(int.from-bytes(1000.to-bytes(endian: "little", size: 5), endian: "little", signed: false), 1000)

--- int-from-and-to-bytes-too-many eval ---
// Error: 2-34 too many bytes to convert to a 64 bit number
#int.from-bytes(bytes((0,) * 16))

--- int-repr eval ---
// Test the `repr` function with integers.
#test(repr(12), "12")
#test(repr(1234567890), "1234567890")
#test(repr(0123456789), "123456789")
#test(repr(0), "0")
#test(repr(-0), "0")
#test(repr(-1), "-1")
#test(repr(-9876543210), "-9876543210")
#test(repr(-0987654321), "-987654321")
#test(repr(4 - 8), "-4")

--- int-display paged ---
// Test integers.
#12 \
#1234567890 \
#0123456789 \
#0 \
#(-0) \
#(-1) \
#(-9876543210) \
#(-0987654321) \
#(4 - 8)

--- issue-int-constructor eval ---
// Test that integer -> integer conversion doesn't do a roundtrip through float.
#let x = 9223372036854775800
#test(type(x), int)
#test(int(x), x)

--- number-invalid-suffix eval ---
// Error: 2-4 invalid number suffix: `u`
#1u
//@@ file tests/suite/foundations_label.typ
// Test labels.

--- label-show-where-selector paged ---
// Test labelled headings.
#show heading: set text(10pt)
#show heading.where(label: <intro>): underline

= Introduction <intro>
The beginning.

= Conclusion
The end.

--- label-after-expression paged ---
// Test label after expression.
#show strong.where(label: <v>): set text(red)

#let a = [*A*]
#let b = [*B*]
#a <v> #b

--- label-dynamic-show-set paged ---
// Test abusing dynamic labels for styling.
#show <red>: set text(red)
#show <blue>: set text(blue)

*A* *B* <red> *C* #label("bl" + "ue") *D*

--- label-after-parbreak paged ---
// Test that label ignores parbreak.
#show <hide>: none

_Hidden_
<hide>

_Hidden_

<hide>
_Visible_

--- label-in-block paged ---
// Test that label only works within one content block.
#show <strike>: strike
// Warning: 13-21 label `<strike>` is not attached to anything
*This is* #[<strike>] *protected.*
*This is not.* <strike>

--- label-unclosed-is-text paged ---
// Test that incomplete label is text.
1 < 2 is #if 1 < 2 [not] a label.

--- label-text-styled-and-sequence eval ---
// Test label on text, styled, and sequence.
#test([Hello<hi>].label, <hi>)
#test([#[A *B* C]<hi>].label, <hi>)
#test([#text(red)[Hello]<hi>].label, <hi>)

--- label-string-conversion eval ---
// Test getting the name of a label.
#test(str(<hey>), "hey")
#test(str(label("hey")), "hey")
#test(str([Hmm<hey>].label), "hey")

--- label-in-code-mode-hint eval ---
// Error: 7-7 expected semicolon or line break
// Hint: 7-7 labels can only be applied in markup mode
// Hint: 7-7 try wrapping your code in a markup block (`[ ]`)
#{ [A] <a> }

--- label-multiple-ignored-warn paged ---
// Warning: 1-8 content labelled multiple times
// Hint: 1-8 only the last label is used, the rest are ignored
= Hello <a> <b>

// Warning: 12-19 content labelled multiple times
// Hint: 12-19 only the last label is used, the rest are ignored
#let f = [#block()<c>]
#f<d>

// Warning: 6-13 content labelled multiple times
// Hint: 6-13 only the last label is used, the rest are ignored
#[#[#block()]<e>]<f>

// Error: 1-3 label `<a>` does not exist in the document
@a

--- label-unattached-warn paged empty ---
#set heading(numbering: "1.")
// Warning: 1-4 label `<a>` is not attached to anything
<a>

--- label-non-existent-error eval ---
// Error: 5-10 sequence does not have field "label"
#[].label

--- label-empty eval ---
// Error: 23-32 label name must not be empty
= Something to label #label("")
//@@ file tests/suite/foundations_panic.typ
// Test the panic function.

--- panic-empty eval ---
// Test panic with no args.
// Error: 2-9 panicked
#panic()

--- panic-int eval ---
// We can panic with a random value.
// Error: 2-12 panicked with: 123
#panic(123)

--- panic-str eval ---
// But usually we panic with an error message.
// Error: 2-24 panicked with: this is wrong
#panic("this is wrong")

--- panic-multiline eval ---
// Test panic with a multiline string.
// Error: 1:2-2:7 panicked with: oops\noops\noops
#panic("oops\noops
oops")

--- panic-raw-newline eval ---
// Test panic with raw text containing escaped and normal newlines.
// Error: 1:2-2:5 panicked with: raw(text: "\\n\n\\n", block: false)
#panic(`\n
\n`)

--- issue-5219-panic-escaped-quotes eval ---
// Error: 2-42 panicked with: use an identifier like "math"
#panic("use an identifier like \"math\"")
//@@ file tests/suite/foundations_path.typ
--- path eval ---
#test(
  repr(path("hi/there.txt")),
  "path(\"/tests/suite/foundations/hi/there.txt\")",
)

--- path-escapes eval ---
// Error: 7-29 path `"../../../../file.txt"` would escape the project root
// Hint: 7-29 cannot access files outside of the project sandbox
// Hint: 7-29 you can adjust the project root with the `--root` argument
#read("../../../../file.txt")

--- path-backslash eval ---
// Error: 7-21 path must not contain a backslash
// Hint: 7-21 use forward slashes instead: `"to/file.txt"`
// Hint: 7-21 in earlier Typst versions, backslashes indicated path separators on Windows
// Hint: 7-21 this behavior is no longer supported as it is not portable
#path("to\\file.txt")
//@@ file tests/suite/foundations_plugin.typ
// Test WebAssembly plugins.

--- plugin-basic eval ---
#let p = plugin("/assets/plugins/hello.wasm")
#test(p.hello(), bytes("Hello from wasm!!!"))
#test(p.double_it(bytes("hey!")), bytes("hey!.hey!"))
#test(
  p.shuffle(bytes("value1"), bytes("value2"), bytes("value3")),
  bytes("value3-value1-value2"),
)

--- plugin-func eval ---
#let p = plugin("/assets/plugins/hello.wasm")
#test(type(p.hello), function)
#test(("a", "b").map(bytes).map(p.double_it), ("a.a", "b.b").map(bytes))

--- plugin-import eval ---
#import plugin("/assets/plugins/hello.wasm"): hello, double_it

#test(hello(), bytes("Hello from wasm!!!"))
#test(double_it(bytes("hey!")), bytes("hey!.hey!"))

--- plugin-transition eval ---
#let empty = plugin("/assets/plugins/hello-mut.wasm")
#test(str(empty.get()), "[]")

#let hello = plugin.transition(empty.add, bytes("hello"))
#test(str(empty.get()), "[]")
#test(str(hello.get()), "[hello]")

#let world = plugin.transition(empty.add, bytes("world"))
#let hello_you = plugin.transition(hello.add, bytes("you"))

#test(str(empty.get()), "[]")
#test(str(hello.get()), "[hello]")
#test(str(world.get()), "[world]")
#test(str(hello_you.get()), "[hello, you]")

#let hello2 = plugin.transition(empty.add, bytes("hello"))
#test(hello == world, false)
#test(hello == hello2, true)

--- plugin-wrong-number-of-arguments eval ---
#let p = plugin("/assets/plugins/hello.wasm")

// Error: 2-20 plugin function takes 0 arguments, but 1 was given
#p.hello(bytes(""))

--- plugin-wrong-argument-type eval ---
#let p = plugin("/assets/plugins/hello.wasm")

// Error: 10-14 expected bytes, found boolean
// Error: 27-29 expected bytes, found integer
#p.hello(true, bytes(()), 10)

--- plugin-error eval ---
#let p = plugin("/assets/plugins/hello.wasm")

// Error: 2-17 plugin errored with: This is an `Err`
#p.returns_err()

--- plugin-panic eval ---
#let p = plugin("/assets/plugins/hello.wasm")

// Error: 2-16 plugin panicked: wasm `unreachable` instruction executed
#p.will_panic()

--- plugin-out-of-bounds-read eval ---
#let p = plugin("/assets/plugins/plugin-oob.wasm")

// Error: 2-14 plugin tried to read out of bounds: pointer 0x40000000 is out of bounds for read of length 1
#p.read_oob()

--- plugin-out-of-bounds-write eval ---
#let p = plugin("/assets/plugins/plugin-oob.wasm")

// Error: 2-27 plugin tried to write out of bounds: pointer 0x40000000 is out of bounds for write of length 3
#p.write_oob(bytes("xyz"))
//@@ file tests/suite/foundations_repr.typ
--- repr eval ---
#let t(a, b) = test(repr(a), b.text)

// Literal values.
#t(auto, `auto`)
#t(true, `true`)
#t(false, `false`)

// Numerical values.
#t(12.0, `12.0`)
#t(3.14, `3.14`)
#t(1234567890.0, `1234567890.0`)
#t(0123456789.0, `123456789.0`)
#t(0.0, `0.0`)
#t(-0.0, `-0.0`)
#t(-1.0, `-1.0`)
#t(-9876543210.0, `-9876543210.0`)
#t(-0987654321.0, `-987654321.0`)
#t(-3.14, `-3.14`)
#t(4.0 - 8.0, `-4.0`)
#t(float.inf, `float.inf`)
#t(-float.inf, `-float.inf`)
#t(float.nan, `float.nan`)

// Strings and escaping.
#t("hi", `"hi"`)
#t("a\n[]\"\u{1F680}string", `"a\n[]\"🚀string"`)

// Array and dictionary.
#t((1, 2, false, ), `(1, 2, false)`)
#t((a: 1, b: "2"), `(a: 1, b: "2")`)

// Functions.
#let f(x) = x
#t(f, `f`)
#t(rect , `rect`)
#t(() => none, `(..) => ..`)
#t(f.with(), `(..) => ..`)

// Types.
#t(int, `int`)
#t(type("hi"), `str`)
#t(type((a: 1)), `dictionary`)

// Constants.
#t(ltr, `ltr`)
#t(left, `left`)

// Content.
#t([*Hey*], `strong(body: [Hey])`)
#t([A _sequence_], `sequence([A], [ ], emph(body: [sequence]))`)
#t([A _longer_ *sequence*!], ```
sequence(
  [A],
  [ ],
  emph(body: [longer]),
  [ ],
  strong(body: [sequence]),
  [!],
)
```)

// Colors and strokes.
#t(rgb("f7a205"), `rgb("#f7a205")`)
#t(2pt + rgb("f7a205"), `2pt + rgb("#f7a205")`)
#t(stroke(), `stroke()`)
#t(stroke(cap: "round", thickness: 2pt), `(thickness: 2pt, cap: "round")`)
#t(blue, `rgb("#0074d9")`)
#t(color.linear-rgb(blue), `color.linear-rgb(0%, 17.46%, 69.39%)`)
#t(oklab(blue), `oklab(56.22%, -0.05, -0.17)`)
#t(oklch(blue), `oklch(56.22%, 0.177, 253.71deg)`)
#t(cmyk(blue), `cmyk(100%, 46.54%, 0%, 14.9%)`)
#t(color.hsl(blue), `color.hsl(207.93deg, 100%, 42.55%)`)
#t(color.hsv(blue), `color.hsv(207.93deg, 100%, 85.1%)`)
#t(luma(blue), `luma(45.53%)`)

// Gradients.
#t(
  gradient.linear(blue, red),
  `gradient.linear((oklab(56.22%, -0.05, -0.17), 0%), (oklab(65.95%, 0.2, 0.108), 100%))`,
)
#t(
  gradient.linear(blue, red, dir: ttb, space: rgb),
  `gradient.linear(dir: rtl, space: rgb, (rgb("#0074d9"), 0%), (rgb("#ff4136"), 100%))`,
)
#t(
  gradient.linear(blue, red, relative: "self", angle: 45deg, space: rgb),
  `gradient.linear(angle: 45deg, space: rgb, relative: "self", (rgb("#0074d9"), 0%), (rgb("#ff4136"), 100%))`,
)
#t(
  gradient.linear(blue, red, space: rgb, angle: 45deg),
  `gradient.linear(angle: 45deg, space: rgb, (rgb("#0074d9"), 0%), (rgb("#ff4136"), 100%))`,
)
//@@ file tests/suite/foundations_std.typ
// Test 'std', a module with the standard library

--- std-basic-access eval ---
#test(std.grid, grid)
#test(std.calc, calc)

--- std-import eval ---
#import std: grid as banana
#test(grid, banana)

--- std-of-shadowed eval ---
#let my-grid = grid[a][b]
#let grid = "oh no!"
#test(my-grid.func(), std.grid)

--- std-shadowing eval ---
#let std = 5
// Error: 6-10 cannot access fields on type integer
#std.grid

--- std-mutation eval ---
// Error: 3-6 cannot mutate a constant: std
#(std = 10)

--- std-shadowed-mutation eval ---
#let std = 10
#(std = 7)
#test(std, 7)

--- std-math paged ---
$ std.rect(x + y = 5) $

--- std-gated-contains-enabled eval features(html) ---
#test("html" in std, true)

--- std-gated-contains-disabled eval features() ---
#test("html" in std, false)
//@@ file tests/suite/foundations_str.typ
// Test the string methods.

--- str-constructor eval ---
// Test conversion to string.
#test(str(123), "123")
#test(str(123, base: 3), "11120")
#test(str(-123, base: 16), "−7b")
#test(str(int.max, base: 36), "1y2p0ij32e8e7")
#test(str(50.14), "50.14")
#test(str(10 / 3).len() > 10, true)

--- str-from-float eval ---
// Test the `str` function with floats.
#test(str(12.0), "12")
#test(str(3.14), "3.14")
#test(str(1234567890.0), "1234567890")
#test(str(0123456789.0), "123456789")
#test(str(0.0), "0")
#test(str(-0.0), "0")
#test(str(-1.0), "−1")
#test(str(-9876543210.0), "−9876543210")
#test(str(-0987654321.0), "−987654321")
#test(str(-3.14), "−3.14")
#test(str(4.0 - 8.0), "−4")

--- str-from-decimal eval ---
// Test the `str` function with decimals.
#test(str(decimal("12")), "12")
#test(str(decimal("12.0")), "12.0")
#test(str(decimal("3.14")), "3.14")
#test(str(decimal("1234567890.0")), "1234567890.0")
#test(str(decimal("0123456789.0")), "123456789.0")
#test(str(decimal("0.0")), "0.0")
#test(str(decimal("-0.0")), "0.0")
#test(str(decimal("-1.0")), "−1.0")
#test(str(decimal("-9876543210.0")), "−9876543210.0")
#test(str(decimal("-0987654321.0")), "−987654321.0")
#test(str(decimal("-3.14")), "−3.14")
#test(str(decimal("-3.9191919191919191919191919195")), "−3.9191919191919191919191919195")
#test(str(decimal("5.0000000000")), "5.0000000000")
#test(str(decimal("4.0") - decimal("8.0")), "−4.0")
#test(str(decimal("4") - decimal("8")), "−4")

--- str-from-int eval ---
// Test the `str` function with integers.
#test(str(12), "12")
#test(str(1234567890), "1234567890")
#test(str(0123456789), "123456789")
#test(str(0), "0")
#test(str(-0), "0")
#test(str(-1), "−1")
#test(str(-9876543210), "−9876543210")
#test(str(-0987654321), "−987654321")
#test(str(4 - 8), "−4")

--- str-constructor-bad-type eval ---
// Error: 6-8 expected integer, float, decimal, version, bytes, label, type, or string, found content
#str([])

--- str-constructor-bad-base eval ---
// Error: 17-19 base must be between 2 and 36
#str(123, base: 99)

--- str-constructor-unary eval ---
// Error: 17-18 base must be between 2 and 36
// Hint: 17-18 generate a unary representation with `"1" * 999`
#str(999, base: 1)

--- str-constructor-unsupported-base eval ---
// Error: 18-19 base is only supported for integers
#str(1.23, base: 2)

--- str-constructor-unsupported-base-ten eval ---
// Error: 18-20 base is only supported for integers
#str(1.23, base: 10)

--- str-from-and-to-unicode eval ---
// Test the unicode function.
#test(str.from-unicode(97), "a")
#test(str.to-unicode("a"), 97)

--- str-from-unicode-bad-type eval ---
// Error: 19-22 expected integer, found content
#str.from-unicode([a])

--- str-to-unicode-bad-type eval ---
// Error: 17-21 expected exactly one character
#str.to-unicode("ab")

--- str-from-unicode-negative eval ---
// Error: 19-21 number must be at least zero
#str.from-unicode(-1)

--- str-from-unicode-bad-value eval ---
// Error: 2-28 0x110000 is not a valid codepoint
#str.from-unicode(0x110000) // 0x10ffff is the highest valid code point

--- str-normalize eval ---
// Test the `normalize` method.
#test("e\u{0301}".normalize(form: "nfc"), "é")
#test("é".normalize(form: "nfd"), "e\u{0301}")
#test("ſ\u{0301}".normalize(form: "nfkc"), "ś")
#test("ſ\u{0301}".normalize(form: "nfkd"), "s\u{0301}")

--- string-len eval ---
// Test the `len` method.
#test("Hello World!".len(), 12)

--- string-first-and-last eval ---
// Test the `first` and `last` methods.
#test("Hello".first(), "H")
#test("Hello".last(), "o")
#test("🏳️‍🌈A🏳️‍⚧️".first(), "🏳️‍🌈")
#test("🏳️‍🌈A🏳️‍⚧️".last(), "🏳️‍⚧️")
#test("hey".first(default: "d"), "h")
#test("".first(default: "d"), "d")
#test("hey".last(default: "d"), "y")
#test("".last(default: "d"), "d")

--- string-first-empty eval ---
// Error: 2-12 string is empty
#"".first()

--- string-last-empty eval ---
// Error: 2-11 string is empty
#"".last()

--- string-at eval ---
// Test the `at` method.
#test("Hello".at(1), "e")
#test("Hello".at(4), "o")
#test("Hello".at(-1), "o")
#test("Hello".at(-2), "l")
#test("Hey: 🏳️‍🌈 there!".at(5), "🏳️‍🌈")

--- string-at-default eval ---
// Test `at`'s 'default' parameter.
#test("z", "Hello".at(5, default: "z"))

--- string-at-not-a-char-boundary eval ---
// Error: 2-14 string index 2 is not a character boundary
#"🏳️‍🌈".at(2)

--- string-at-out-of-bounds eval ---
// Error: 2-15 no default value was specified and string index out of bounds (index: 5, len: 5)
#"Hello".at(5)

--- string-at-at-default-other-type eval ---
#test("Hello".at(5, default: (a: 10)), (a: 10))

--- string-slice eval ---
// Test the `slice` method.
#test("abc".slice(1, 2), "b")
#test("abc🏡def".slice(2, 7), "c🏡")
#test("abc🏡def".slice(2, -2), "c🏡d")
#test("abc🏡def".slice(-3, -1), "de")
#test("x🏡yz".slice(-2, count: 2), "yz")
#test("x🏡yz".slice(-7, count: 7), "x🏡yz")

--- string-slice-count-end eval ---
// Error: 2-29 `end` and `count` are mutually exclusive
#"abc".slice(0, 1, count: 2)

--- string-slice-not-a-char-boundary eval ---
// Error: 2-21 string index -1 is not a character boundary
#"🏳️‍🌈".slice(0, -1)

--- string-clusters eval ---
// Test the `clusters` and `codepoints` methods.
#test("abc".clusters(), ("a", "b", "c"))
#test("abc".clusters(), ("a", "b", "c"))
#test("🏳️‍🌈!".clusters(), ("🏳️‍🌈", "!"))

--- string-codepoints eval ---
#test("🏳️‍🌈!".codepoints(), ("🏳", "\u{fe0f}", "\u{200d}", "🌈", "!"))

--- string-contains eval ---
// Test the `contains` method.
#test("abc".contains("b"), true)
#test("b" in "abc", true)
#test("1234f".contains(regex("\\d")), true)
#test(regex("\\d") in "1234f", true)
#test("abc".contains("d"), false)
#test("1234g" in "1234f", false)
#test("abc".contains(regex("^[abc]$")), false)
#test("abc".contains(regex("^[abc]+$")), true)

--- string-starts-with eval ---
// Test the `starts-with` and `ends-with` methods.
#test("Typst".starts-with("Ty"), true)
#test("Typst".starts-with(regex("[Tt]ys")), false)
#test("Typst".starts-with("st"), false)

--- string-ends-with eval ---
#test("Typst".ends-with("st"), true)
#test("Typst".ends-with(regex("\\d*")), true)
#test("Typst".ends-with(regex("\\d+")), false)
#test("Typ12".ends-with(regex("\\d+")), true)
#test("typst13".ends-with(regex("1[0-9]")), true)
#test("typst113".ends-with(regex("1[0-9]")), true)
#test("typst23".ends-with(regex("1[0-9]")), false)

--- string-find-and-position eval ---
// Test the `find` and `position` methods.
#let date = regex("\\d{2}:\\d{2}")
#test("Hello World".find("World"), "World")
#test("Hello World".position("World"), 6)
#test("It's 12:13 now".find(date), "12:13")
#test("It's 12:13 now".position(date), 5)

--- string-match eval ---
// Test the `match` method.
#test("Is there a".match("for this?"), none)
#test(
  "The time of my life.".match(regex("[mit]+e")),
  (start: 4, end: 8, text: "time", captures: ()),
)

--- string-matches eval ---
// Test the `matches` method.
#test("Hello there".matches("\d"), ())
#test("Day by Day.".matches("Day"), (
  (start: 0, end: 3, text: "Day", captures: ()),
  (start: 7, end: 10, text: "Day", captures: ()),
))

// Compute the sum of all timestamps in the text.
#let timesum(text) = {
  let time = 0
  for match in text.matches(regex("(\\d+):(\\d+)")) {
    let caps = match.captures
    time += 60 * int(caps.at(0)) + int(caps.at(1))
  }
  str(int(time / 60)) + ":" + str(calc.rem(time, 60))
}

#test(timesum(""), "0:0")
#test(timesum("2:70"), "3:10")
#test(timesum("1:20, 2:10, 0:40"), "4:10")

--- string-replace eval ---
// Test the `replace` method with `Str` replacements.
#test("ABC".replace("", "-"), "-A-B-C-")
#test("Ok".replace("Ok", "Nope", count: 0), "Ok")
#test("to add?".replace("", "How ", count: 1), "How to add?")
#test("AB C DEF GH J".replace(" ", ",", count: 2), "AB,C,DEF GH J")
#test("Walcemo"
  .replace("o", "k")
  .replace("e", "o")
  .replace("k", "e")
  .replace("a", "e"),
  "Welcome"
)
#test("123".replace(regex("\\d$"), "_"), "12_")
#test("123".replace(regex("\\d{1,2}$"), "__"), "1__")

--- string-replace-function eval ---
// Test the `replace` method with `Func` replacements.

#test("abc".replace(regex("[a-z]"), m => {
  str(m.start) + m.text + str(m.end)
}), "0a11b22c3")
#test("abcd, efgh".replace(regex("\\w+"), m => {
  upper(m.text)
}), "ABCD, EFGH")
#test("hello : world".replace(regex("^(.+)\\s*(:)\\s*(.+)$"), m => {
  upper(m.captures.at(0)) + m.captures.at(1) + " " + upper(m.captures.at(2))
}), "HELLO : WORLD")
#test("hello world, lorem ipsum".replace(regex("(\\w+) (\\w+)"), m => {
  m.captures.at(1) + " " + m.captures.at(0)
}), "world hello, ipsum lorem")
#test("hello world, lorem ipsum".replace(regex("(\\w+) (\\w+)"), count: 1, m => {
  m.captures.at(1) + " " + m.captures.at(0)
}), "world hello, lorem ipsum")
#test("123 456".replace(regex("[a-z]+"), "a"), "123 456")

#test("abc".replace("", m => "-"), "-a-b-c-")
#test("abc".replace("", m => "-", count: 1), "-abc")
#test("123".replace("abc", m => ""), "123")
#test("123".replace("abc", m => "", count: 2), "123")
#test("a123b123c".replace("123", m => {
  str(m.start) + "-" + str(m.end)
}), "a1-4b5-8c")
#test("halla warld".replace("a", m => {
  if m.start == 1 { "e" }
  else if m.start == 4 or m.start == 7 { "o" }
}), "hello world")
#test("aaa".replace("a", m => str(m.captures.len())), "000")

--- string-replace-function-bad-type eval ---
// Error: 23-29 expected string, found integer
#"123".replace("123", m => 1)

--- string-replace-bad-type eval ---
// Error: 23-32 expected string or function, found array
#"123".replace("123", (1, 2, 3))

--- string-trim-basic eval ---
// Test the `trim` method; the pattern is not provided.
#let str = "Typst, LaTeX, Word, InDesign"
#let array = ("Typst", "LaTeX", "Word", "InDesign")
#test(str.split(",").map(s => s.trim()), array)
#test("".trim(), "")
#test("  ".trim(), "")
#test("\t".trim(), "")
#test("\n".trim(), "")
#test("\t \n".trim(), "")
#test(" abc ".trim(at: start), "abc ")
#test("\tabc ".trim(at: start), "abc ")
#test("abc\n".trim(at: end), "abc")
#test(" abc ".trim(at: end, repeat: true), " abc")
#test("  abc".trim(at: start, repeat: false), "abc")

--- string-trim-pattern-str eval ---
// Test the `trim` method; the pattern is a string.
#test("aabcaa".trim("a", repeat: false), "abca")
#test("aabca".trim("a", at: start), "bca")
#test("aabcaa".trim("a", at: end, repeat: false), "aabca")
#test(" abc\n".trim("\n"), " abc")
#test("whole".trim("whole", at: start), "")

--- string-trim-pattern-regex eval ---
// Test the `trim` method; the pattern is a regex.
#test("".trim(regex(".")), "")
#test("123abc456".trim(regex("\\d")), "abc")
#test("123abc456".trim(regex("\\d"), repeat: false), "23abc45")
#test("123a4b5c678".trim(regex("\\d"), repeat: true), "a4b5c")
#test("123a4b5c678".trim(regex("\\d"), repeat: false), "23a4b5c67")
#test("123abc456".trim(regex("\\d"), at: start), "abc456")
#test("123abc456".trim(regex("\\d"), at: end), "123abc")
#test("123abc456".trim(regex("\\d+"), at: end, repeat: false), "123abc")
#test("123abc456".trim(regex("\\d{1,2}$"), repeat: false), "123abc4")
#test("hello world".trim(regex(".")), "")
#test("12306".trim(regex("\\d"), at: start), "")
#test("12306abc".trim(regex("\\d"), at: start), "abc")
#test("whole".trim(regex("whole"), at: start), "")
#test("12306".trim(regex("\\d"), at: end), "")
#test("abc12306".trim(regex("\\d"), at: end), "abc")
#test("whole".trim(regex("whole"), at: end), "")

--- string-trim-at-bad-alignment eval ---
// Error: 17-21 expected either `start` or `end`
#"abc".trim(at: left)

--- string-regex-backslash eval ---
// Using single/double backslash will change semantical meaning of a valid
// string escape sequence (ES).
#let remove(source, pattern) = source.replace(regex(pattern), "")
#test(remove("
", "\n"), "") // Literal newline (Line Feed) in source, ES in pattern (1 byte).
#test(remove("
", "\\n"), "") // The `\n` regex token that represents newline (2 bytes).
#test(remove("\n", "\n"), "") // ES in the source string.
#test(remove("\n", "\\n"), "") // ES in the source string.
#test(remove("\\\t\n1", "\\\\\\t\\n\\d"), "") // Proper string-based regex.
#test(remove("\\\t\n1", `\\\t\n\d`.text), "") // Better raw-based approach.
#test(remove(" word-wordle", "\bword\b"), " -wordle") // Invalid ES.
#test(remove(" word-wordle", "\\bword\\b"), " -wordle") // Valid regex tokens.

--- string-split eval ---
// Test the `split` method.
#test("abc".split(""), ("", "a", "b", "c", ""))
#test("abc".split("b"), ("a", "c"))
#test("a123c".split(regex("\\d")), ("a", "", "", "c"))
#test("a123c".split(regex("\\d+")), ("a", "c"))

--- string-rev eval ---
// Test the `rev` method.
#test("abc".rev(), "cba")
#test("ax̂e".rev(), "ex̂a")

--- string-unclosed eval ---
// Error: 1:2-2:1 unclosed string
#"hello\"
//@@ file tests/suite/foundations_type.typ
--- type eval ---
#test(type(1), int)
#test(type(ltr), direction)
#test(type(10 / 3), float)

--- issue-3110-type-constructor eval ---
// Let the error message report the type name.
// Error: 2-9 type content does not have a constructor
#content()

--- issue-3110-associated-field eval ---
// Error: 6-12 type integer does not contain field `MAXVAL`
#int.MAXVAL

--- issue-3110-associated-function eval ---
// Error: 6-18 type string does not contain field `from-unïcode`
#str.from-unïcode(97)

--- issue-2747-repr-auto-none eval ---
#test(repr(none), "none")
#test(repr(auto), "auto")
#test(repr(type(none)), "type(none)")
#test(repr(type(auto)), "type(auto)")
//@@ file tests/suite/foundations_version.typ
// Test versions.

--- version-constructor eval ---
// Test version constructor.

// Empty.
#test(array(version()), ())

// Plain.
#test(version(1, 2).major, 1)

// Single Array argument.
#test(version((1, 2)).minor, 2)

// Mixed arguments.
#test(version(1, (2, 3), 4, (5, 6), 7).at(5), 6)

--- version-equality eval ---
// Test equality of different-length versions
#test(version(), version(0))
#test(version(0), version(0, 0))
#test(version(1, 2), version(1, 2, 0, 0, 0, 0))

--- version-at eval ---
// Test `version.at`.

// Non-negative index in bounds
#test(version(1, 2).at(1), 2)

// Non-negative index out of bounds
#test(version(1, 2).at(4), 0)

// Negative index in bounds
#test(version(1, 2).at(-2), 1)

// Error: 2-22 component index out of bounds (index: -3, len: 2)
#version(1, 2).at(-3)

--- version-fields eval ---
// Test version fields.
#test(version(1, 2, 3).major, 1)
#test(version(1, 2, 3).minor, 2)
#test(version(1, 2, 3).patch, 3)

--- version-type eval ---
// Test the type of `sys.version`
#test(type(sys.version), version)
//@@ file tests/suite/html_elem.typ
--- html-elem-alone-context html ---
#context html.elem("html")

--- html-elem-not-alone html ---
// Error: 2-19 `<html>` element must be the only element in the document
#html.elem("html")
Text

--- html-elem-metadata html ---
#html.elem("html", context {
  let val = query(<l>).first().value
  test(val, "Hi")
  val
})
#metadata("Hi") <l>

--- html-elem-custom html ---
#html.elem("my-element")[Hi]
#html.elem("custom-button")[Hi]
#html.elem("multi-word-component")[Hi]
#html.elem("element-")[Hi]

--- html-elem-attrs-fold html ---
#set html.elem(attrs: (a: "1"))
#set html.elem(attrs: (c: "3"))
#set html.elem(attrs: (d: "4"))
#set html.elem(attrs: (c: "3'"))
#html.elem("div", attrs: (a: "1'", b: "2"))

--- html-elem-invalid eval ---
// Error: 12-24 the character "@" is not valid in a tag name
#html.elem("my@element")

--- html-elem-custom-bad-start eval ---
// Error: 12-22 custom element name must start with a lowercase letter
#html.elem("1-custom")

--- html-elem-custom-uppercase eval ---
// Error: 12-21 custom element name must not contain uppercase letters
#html.elem("my-ELEM")

--- html-elem-custom-reserved eval ---
// Error: 12-28 name is reserved and not valid for a custom element
#html.elem("annotation-xml")
//@@ file tests/suite/html_frame.typ
--- html-frame html ---
A rectangle:
#html.frame(rect())

--- html-frame-in-layout paged ---
// Ensure that HTML frames are transparent in layout. This is less important for
// actual paged export than for _nested_ HTML frames, which take the same code
// path.
#html.frame[A]

--- html-frame-position html ---
// Test that positions are available within a frame, but nowhere else.
#context test(here().position(), none)
#html.frame(box(width: 100pt, height: 50pt, {
  place(top + left, dx: 30pt, dy: 20pt, context {
    test(here().position(), (x: 30pt, y: 20pt))
  })
}))
//@@ file tests/suite/html_mathml.typ
--- mathml-dif html ---
$ integral dif x Dif y $

--- mathml-vary-class-spacing html ---
$ x = - 1 quad x - 1 $

--- mathml-text-op html ---
$ tan x = (sin x)/(cos x) $
$ op("custom", limits: #true)_(n -> oo) n $

--- mathml-lr html ---
$ (x/2 + z) $

--- mathml-lr-size html ---
$ lr((a + b), size: #5em) $

--- mathml-spacing-resolve html ---
$
  a #h(1em + 10pt) b \
  c #h(2em) d \
  e #h(1em) #h(10pt, weak: true) e
$

--- mathml-equation-alignment-spacing html ---
$
    a & = b + c \
  d + & - f
$

--- mathml-equation-alignment html ---
$
  a & = b + c \
    & = d + e \
    & = f + g
$

--- mathml-multiline-alignment-2 html ---
$
  a_n & = a \
      & = a
$

--- mathml-multiline-alignment-3 html ---
$
  a + b & = c \
      d & = e + f
$

--- mathml-multiline-matrix-spacing html ---
$
  "square matrix" & = mat(1, 0; 0, 1) \
          "array" & = mat(1, 0) \
         "vector" & = vec(1, 0)
$

--- mathml-stretch-largeop-vs-explicit html ---
$ integral $
$ stretch(integral) $
$ stretch(integral, size: #50pt) $
$ stretch(integral, size: #50%) $

--- mathml-stretch-vertical-alignment html ---
$ stretch(\[, size: #4em) a/b stretch(arrow.t, size: #4em) $

--- mathml-stretch html ---
$ a -> b $
$ a stretch(->) b $
$ a stretch(->, size: #3em) b $
$ a stretch(->, size: #200%) b $

--- mathml-custom-class html ---
#let loves = math.class(
  "relation",
  sym.suit.heart,
)
$x loves y and y loves 5$

--- mathml-primes html ---
$ x' x'' x''' x'''' x''''' x'''''' $

--- mathml-primes-factorial-explicit-class html ---
$ n! + (n+1)! + class("punctuation", !) + class("opening", !) + ! $
$ n' + (n+1)' + class("punctuation", ') + class("opening", ') + ' $

--- mathml-ignored-math-content html ---
#set math.frac(style: "skewed")
$
  // Warning: 7-16 cancel was ignored during MathML export
  // Warning: 19-30 overline was ignored during MathML export
  // Warning: 19-32 skewed fraction was ignored during MathML export
  // Warning: 35-47 underline was ignored during MathML export
  a + cancel(x) + overline(y)/d = underline(x) - 1
$

--- mathml-ignored-external-content html ---
$
  // Warning: 23-30 grid was ignored during HTML export
  a + #box[b] + c != #grid[d]
$

--- mathml-inline-multiline-equation html ---
Blah $a \ b$ blah.

Blah $a &= b + c \ d + e &= f$ blah.

Blah $a + b + c \ d$ blah.

--- mathml-attach-single-op html ---
$ f^+ $
$ f^times $
$ ZZ^(plus.o r) ZZ^(plus.o.big r) $

--- mathml-box-equation html ---
A #box($ sum_(i = 0)^oo $) B #box($sum_(i = 0)^oo$) C

--- mathml-block-equation html ---
A #block($sum_(i = 0)^oo$) B #block($ sum_(i = 0)^oo $) C

--- mathml-box-block-equation-nested html ---
#block(box($ sum_(i = 0)^oo $)) vs #box(block($sum_(i = 0)^oo$))

--- mathml-html-elem-styles html ---
#show math.attach: set bibliography(title: "Hi")
#show html.elem.where(tag: "mi"): context bibliography.title
$ a^+ $

--- mathml-html-elem-show-rule html ---
#show html.elem.where(tag: "mo"): none
$ a^(1+2) $

--- mathml-show-rule html ---
#show math.frac: it => html.elem("mrow", it)
$ a/b $

--- mathml-show-rule-paged paged empty ---
// Warning: 24-45 MathML element was ignored during paged export
#show math.frac: it => html.elem("mrow", it)
$ a/b $

--- mathml-nested-elem-show-rule html ---
#set text(10pt)
#show math.attach: set text(size: 2em)
#show html.elem.where(tag: "mi"): context repr(text.size)
$ (a)^+ $

--- mathml-non-math-in-math html ---
#html.elem("mtext")[*bold* text]
#html.elem("mtext", html.div())

--- mathml-show-rule-non-math-in-math html ---
#show math.attach: html.elem("mn")[*bold* text]
$ a^1 $

--- mathml-show-rule-non-math-in-math-in-math html ---
#show html.elem.where(tag: "msqrt"): html.elem("mtext")[*bold* text]
$ a/sqrt(b) $

--- mathml-show-rule-math-in-non-math-in-math html ---
#show html.elem.where(tag: "msqrt"): html.elem("div", $x^2$)
$ a/sqrt(b) $

--- mathml-custom-head-stylesheet html ---
#html.html({
  html.head()
  html.body[
    Hello $x$
  ]
})

--- mathml-show-rule-accent html ---
#show math.overline: it => {
  html.elem("mover", attrs: (accent: "true"), {
    html.elem("mrow", it.body)
    $class("diacritic", stretch(\u{203E}))$
  })
}
$ overline(A B C) $
//@@ file tests/suite/html_syntax.typ
--- html-void-element-with-children html ---
// Error: 2-27 HTML void elements must not have children
#html.elem("img", [Hello])

--- html-space-collapsing html ---
// Note: <s>..</s> = <span style="white-space: pre-wrap">..</span>
#import html: span

= Single spaces
// No collapsing.
#"A B"
// -> A B

// No collapsing, multiple text elements.
#"A"#" "#"B"
// -> A B

// Across span boundaries: 0-1.
#span[A] B
// -> <span>A</span> B

// With span in between.
#"A "#span()#" B"
// -> A<s> </s><span></span> B

// With metadata in between.
#"A "#metadata(none)#" B"
// -> A<s>  </s>B

// Within span.
#span("A ")B
// -> <span>A </span>B

= Consecutive whitespace
// Single text element.
#"A  B   C"
// -> A<s>  </s>B<s>   </s>C

// Multiple text elements.
A#"  "B#"   C"
// -> A<s>  </s>B<s>   </s>C

// Across span boundaries: 1-1.
#span("A ") B
// -> <span>A<s> </s></span> B

// Across span boundaries: 1-2.
#span("A ")#"  B"
// -> <span>A </span><s>  </s>B

// Across span boundaries: 2-1.
#span("A  ") B
// -> <span>A<s>  </s></span> B

// Across span boundaries: 2-2.
#span("A  ")#"  B"
// -> <span>A<s>  </s></span><s>  </s>B

// With span in between.
#"A  "#span()#"  B"
// -> A<s>  </s><span></span><s>  </s>B

// With metadata in between.
#"A "#metadata(none)#"  B"
// -> A<s>   </s>B

= Leading whitespace
// Leading space.
#" A"
// -> <s> </s>A

// Leading space in span.
#span(" ")A
// -> <span><s> </s></span>A

// Leading space with preceding empty element.
#span()#" "A
// -> <span></span><s> </s>A

= Trailing whitespace
// Trailing space.
#"A "
// -> A<s> </s>

// Trailing space in element.
#span("A ")
// -> A<span><s> </s></span>

// Trailing space in element with following empty element.
#span("A ")#span()
// -> <span>A<s> </s></span><span></span>

= Tabs
// Single text element.
#"A\tB"
// -> A<s>&#9;</s>B

// Multiple text elements.
#"A"#"\t"#"B"
// -> A<s>&#9;</s>B

// Spaces + Tab.
#"A \t B"
// -> A<s> &#9; </s>B

= Newlines
// Normal line feed.
#"A\nB"
// -> A<br>B

// CRLF.
#"A\r\nB"
// -> A<br>B

// Spaces + newline.
#"A \n B"
// -> A<s> </s><br><s> </s>B

// Explicit `<br>` element.
#"A "#html.br()#" B"
// -> A<s> </s><br><s> </s>B

// Newline in span.
#"A "#span("\n")#" B"
// -> A<s> </s><span><br></span><s> </s>B

= With default ignorables
// With default ignorable in between.
#"A \u{200D} B"
// -> A<s> </s>&#x200D; B

#"A  \u{200D}  B"
// -> A<s>  </s>&#x200D;<s>  </s>B

= Everything
// Everything at once.
#span("  A ")#"\r\n\t"B#" "#span()
// -> <span><s>  </s>A<s> </s></span><br><s>&#9;</s>B<s> </s><span></span>

= Special
// Escapable raw.
#html.textarea("A  B")
// -> <textarea>A  B</textarea>

// Preformatted.
#html.pre("A  B")
// -> <pre>A  B</pre>

--- html-space-protection-replaced html ---
A #html.input(type: "text")

--- html-space-protection-br html ---
A #html.br() B

--- html-space-collapsing-mixed-flow html ---
// Test that elements that have `display: block` by default
// don't force space protection in a mixed flow.
#block("A" + [ ] + html.div[B])
#block(html.div[A] + [ ] + "B")

--- html-pre-starting-with-newline html ---
#html.pre("hello")
#html.pre("\nhello")
#html.pre("\n\nhello")

--- html-textarea-starting-with-newline html ---
#html.textarea("\nenter")

--- html-script html ---
// This should be pretty and indented.
#html.script(
  ```js
  const x = 1
  const y = 2
  console.log(x < y, Math.max(1, 2))
  ```.text,
)

// This should have extra newlines, but no indent because of the multiline
// string literal.
#html.script("console.log(`Hello\nWorld`)")

// This should be untouched.
#html.script(
  type: "text/python",
  ```py
  x = 1
  y = 2
  print(x < y, max(x, y))
  ```.text,
)

--- html-style html ---
// This should be pretty and indented.
#html.style(
  ```css
  body {
    text: red;
  }
  ```.text,
)

--- html-style-str html ---
#html.style("body { --name: \"Typst\"; }")

--- html-style-content eval ---
// Error: 1:12-3:2 expected string, found content
#html.style[
  body { --name: "Typst"; }
]

--- html-untyped-style-content html ---
#html.elem("style")[
  body { --name: "Typst"; }
]

--- html-raw-text-contains-elem eval ---
// Error: 14-32 expected string, found content
#html.script(html.strong[Hello])

--- html-raw-text-contains-frame html ---
// Error: 22-39 HTML raw text element cannot have non-text children
#html.elem("script", html.frame[Hello])

--- html-raw-text-contains-closing-tag html ---
// Error: 2-32 HTML raw text element cannot contain its own closing tag
// Hint: 2-32 the sequence `</SCRiPT` appears in the raw text
#html.script("hello </SCRiPT ")

--- html-escapable-raw-text-contains-elem html ---
// Error: 16-34 HTML raw text element cannot have non-text children
#html.textarea(html.strong[Hello])

--- html-escapable-raw-text-contains-closing-tag html ---
// This is okay because we escape it.
#html.textarea("hello </textarea>")

--- html-non-char html ---
// Error: 1-9 the character `"\u{fdd0}"` cannot be encoded in HTML
\u{fdd0}

--- html-raw-text-non-char html ---
// Error: 14-34 the character `"\u{fdd0}"` cannot be encoded in HTML
#html.script("const x = \u{fdd0}")

--- html-escapable-raw-text-non-char html ---
// Error: 23-31 the character `"\u{fdd0}"` cannot be encoded in HTML
#html.textarea[Typing \u{fdd0}]

--- issue-8725-html-raw-multi-byte-char html ---
#html.elem("style", "</abcd−")
//@@ file tests/suite/html_typed.typ
--- html-typed html ---
// String
#html.div(id: "hi")

// Different kinds of options.
#html.div(aria-autocomplete: none) // "none"
#html.div(aria-expanded: none) // "undefined"
#html.link(referrerpolicy: none) // present

// Different kinds of bools.
#html.div(autofocus: false) // absent
#html.div(autofocus: true) // present
#html.div(hidden: false) // absent
#html.div(hidden: true) // present
#html.div(aria-atomic: false) // "false"
#html.div(aria-atomic: true) // "true"
#html.div(translate: false) // "no"
#html.div(translate: true) // "yes"
#html.form(autocomplete: false) // "on"
#html.form(autocomplete: true) // "off"

// Char
#html.div(accesskey: "K")

// Int
#html.div(aria-colcount: 2)
#html.object(width: 120, height: 10)
#html.td(rowspan: 2)

// Float
#html.meter(low: 3.4, high: 7.9)

// Space-separated strings.
#html.div(class: "alpha")
#html.div(class: "alpha beta")
#html.div(class: ("alpha", "beta"))

// Comma-separated strings.
#html.div(html.input(accept: "image/jpeg"))
#html.div(html.input(accept: "image/jpeg, image/png"))
#html.div(html.input(accept: ("image/jpeg", "image/png")))

// Comma-separated floats.
#html.area(coords: (2.3, 4, 5.6))

// Colors.
#for c in (
  red,
  red.lighten(10%),
  luma(50%),
  cmyk(10%, 20%, 30%, 40%),
  oklab(27%, 20%, -3%, 50%),
  color.linear-rgb(20%, 30%, 40%, 50%),
  color.hsl(20deg, 10%, 20%),
  color.hsv(30deg, 20%, 30%),
) {
  html.link(color: c)
}

// Durations & datetimes.
#for d in (
  duration(weeks: 3, seconds: 4),
  duration(days: 1, minutes: 4),
  duration(),
  datetime(day: 10, month: 7, year: 2005),
  datetime(day: 1, month: 2, year: 0),
  datetime(hour: 6, minute: 30, second: 0),
  datetime(day: 1, month: 2, year: 0, hour: 11, minute: 11, second: 0),
  datetime(day: 1, month: 2, year: 0, hour: 6, minute: 0, second: 9),
) {
  html.div(html.time(datetime: d))
}

// Direction
#html.div(dir: ltr)[RTL]

// Image candidate and source size.
#html.img(
  src: "image.png",
  alt: "My wonderful image",
  srcset: (
    (src: "/image-120px.png", width: 120),
    (src: "/image-60px.png", width: 60),
  ),
  sizes: (
    (condition: "min-width: 800px", size: 400pt),
    (condition: "min-width: 400px", size: 250pt),
  )
)

// String enum.
#html.form(enctype: "text/plain")
#html.form(role: "complementary")
#html.div(hidden: "until-found")

// Or.
#html.div(aria-checked: false)
#html.div(aria-checked: true)
#html.div(aria-checked: "mixed")

// Input value.
#html.div(html.input(value: 5.6))
#html.div(html.input(value: red))
#html.div(html.input(min: 3, max: 9))

// Icon size.
#html.link(rel: "icon", sizes: ((32, 24), (64, 48)))

--- html-typed-dir-str eval ---
// Error: 16-21 expected direction or auto, found string
#html.div(dir: "ltr")

--- html-typed-char-too-long eval ---
// Error: 22-35 expected exactly one character
#html.div(accesskey: ("Ctrl", "K"))

--- html-typed-int-negative eval ---
// Error: 18-21 number must be at least zero
#html.img(width: -10)

--- html-typed-int-zero eval ---
// Error: 22-23 number must be positive
#html.textarea(rows: 0)

--- html-typed-float-negative eval ---
// Error: 19-23 number must be positive
#html.input(step: -3.4)

--- html-typed-string-array-with-space eval ---
// Error: 18-41 array item may not contain a space
// Hint: 18-41 the array attribute will be encoded as a space-separated string
#html.div(class: ("alpha beta", "gamma"))

--- html-typed-float-array-invalid-shorthand eval ---
// Error: 20-23 expected array, found float
#html.area(coords: 4.5)

--- html-typed-dir-vertical eval ---
// Error: 16-19 direction must be horizontal
#html.div(dir: ttb)

--- html-typed-string-enum-invalid eval ---
// Error: 21-28 expected "application/x-www-form-urlencoded", "multipart/form-data", or "text/plain"
#html.form(enctype: "utf-8")

--- html-typed-or-invalid eval ---
// Error: 25-31 expected boolean or "mixed"
#html.div(aria-checked: "nope")

--- html-typed-string-enum-or-array-invalid eval ---
// Error: 27-33 expected array, "additions", "additions text", "all", "removals", or "text"
// Error: 49-54 expected boolean or "mixed"
#html.link(aria-relevant: "nope", aria-checked: "yes")

--- html-typed-srcset-both-width-and-density eval ---
// Error: 19-64 cannot specify both `width` and `density`
#html.img(srcset: ((src: "img.png", width: 120, density: 0.5),))

--- html-typed-srcset-src-comma eval ---
// Error: 19-50 `src` must not start or end with a comma
#html.img(srcset: ((src: "img.png,", width: 50),))

--- html-typed-sizes-string-size eval ---
// Error: 18-66 expected length, found string
// Hint: 18-66 CSS lengths that are not expressible as Typst lengths are not yet supported
// Hint: 18-66 you can use `html.elem` to create a raw attribute
#html.img(sizes: ((condition: "min-width: 100px", size: "10px"),))

--- html-typed-input-value-invalid eval ---
// Error: 20-25 expected string, float, datetime, color, or array, found boolean
#html.input(value: false)

--- html-typed-input-bound-invalid eval ---
// Error: 18-21 expected string, float, or datetime, found color
#html.input(min: red)

--- html-typed-icon-size-invalid eval ---
// Error: 32-45 expected array, found string
#html.link(rel: "icon", sizes: "10x20 20x30")

--- html-typed-hidden-none eval ---
// Error: 19-23 expected boolean or "until-found", found none
#html.div(hidden: none)

--- html-typed-invalid-body eval ---
// Error: 10-14 unexpected argument
#html.img[hi]
//@@ file tests/suite/introspection_convergence.typ
// Tests convergence warnings.

--- convergence-query paged ---
// Warning: document did not converge within five attempts
// Hint: see 5 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
#show strong: none

= Real <real>

#context {
  // Warning: 15-29 number of heading elements did not stabilize
  // Hint: 15-29 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 1\n- run 3: 2\n- run 4: 3\n- run 5: 4\n- final: 5
  let elems = query(heading)
  let count = elems.len()
  count * [= Fake <fake>]
}

#context {
  // This one converges.
  _ = query(<real>)
}

// Test alternative warning messages.
#context {
  // Warning: 7-37 number of matching heading elements did not stabilize
  // Hint: 7-37 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 1\n- run 3: 2\n- run 4: 3\n- run 5: 4\n- final: 5
  _ = query(heading.where(level: 1))

  // Warning: 7-20 number of elements labelled `<fake>` did not stabilize
  // Hint: 7-20 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 0\n- run 3: 1\n- run 4: 2\n- run 5: 3\n- final: 4
  _ = query(<fake>)

  // Warning: 7-48 number of elements matching `selector.or(<fake>, heading.where(level: 1))` did not stabilize
  // Hint: 7-48 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 1\n- run 3: 2\n- run 4: 3\n- run 5: 4\n- final: 5
  _ = query(heading.where(level: 1).or(<fake>))
}

// This one has no hint since the number of matching elements is the same and
// it's difficult to provide a good hint for the concrete elements.
#switch(n => if n == 4 [*A*] else [*B*])
#context {
  // Warning: 7-20 query for strong elements did not stabilize
  _ = query(strong)
}

--- convergence-query-first-and-unique paged ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
#switch(n => if n == 4 [*A* <a>] else if n == 5 [_A_ <a>])

// Warning: 2-17 query for a unique element labelled `<a>` did not stabilize
// Warning: 2-17 query for the first element matching `location(..)` did not stabilize
// Hint: 2-17 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 0\n- run 3: 0\n- run 4: 0\n- run 5: 1\n- final: 0
#link(<a>)[Link]

--- convergence-query-label paged ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
#set heading(numbering: "1.")
= A

// Warning: 25-28 value of `counter(heading)` did not converge
// Hint: 25-28 the following values were observed:\n- run 1: 0\n- run 2: 1\n- run 3: 1\n- run 4: 1\n- run 5: 1\n- final: 2
#switch(n => if n == 5 [= B <b>])

// Error: 1-3 label `<b>` does not exist in the document
// Warning: 1-3 query for a unique element labelled `<b>` did not stabilize
// Hint: 1-3 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 0\n- run 3: 0\n- run 4: 0\n- run 5: 0\n- final: 1
@b

--- convergence-position paged ---
// Warning: document did not converge within five attempts
// Hint: see 1 additional warning for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch

// Hint: 29-38 heading was created here
#switch(n => v(n * 10pt) + [= Heading])

// Warning: 10-36 heading position did not stabilize
// Hint: 10-36 the following positions were observed:\n- run 1: none\n- run 2: page 1 at (10pt, 20pt)\n- run 3: page 1 at (10pt, 30pt)\n- run 4: page 1 at (10pt, 40pt)\n- run 5: page 1 at (10pt, 50pt)\n- final: page 1 at (10pt, 60pt)
#context locate(heading).position().y

--- convergence-page paged ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch

// No "heading was created here" hint because the heading does not exist anymore
// in the end.
#switch(n => if n == 4 { pagebreak() + [= Heading] })

// Warning: 10-25 query for a unique heading element did not stabilize
// Hint: 10-25 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 0\n- run 3: 0\n- run 4: 0\n- run 5: 1\n- final: 0
// Warning: 10-32 page number of the element did not stabilize
// Hint: 10-32 the following page numbers were observed:\n- run 1: none\n- run 2: none\n- run 3: none\n- run 4: none\n- run 5: page 2\n- final: none
#context locate(heading).page()

--- convergence-page-supplement paged ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch

#set page(numbering: "1", margin: (bottom: 20pt))
#show: doc => switch(n => {
  set page(supplement: "Pagus", numbering: "I") if n == 4
  doc
})

// Hint: 1-8 heading was created here
// Hint: 1-8 heading was created here
= Hello <hello>

// Warning: 2-28 supplement of the page on which the heading is located did not stabilize
// Warning: 2-28 numbering of the page on which the heading is located did not stabilize
// Hint: 2-28 the following supplements were observed:\n- run 1: `[]`\n- run 2: `[page]`\n- run 3: `[page]`\n- run 4: `[page]`\n- run 5: `[Pagus]`\n- final: `[page]`
// Hint: 2-28 the following numberings were observed:\n- run 1: `none`\n- run 2: `"1"`\n- run 3: `"1"`\n- run 4: `"1"`\n- run 5: `"I"`\n- final: `"1"`
#ref(<hello>, form: "page")

--- convergence-state paged empty ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help

#let hi = state("hi", 0)
#context hi.update(hi.get() + 1)
#context hi.update(hi.get() + 2)
#context hi.update(hi.get() + 3)
#context hi.update(hi.get() + 4)
#context hi.update(hi.get() + 5)

// Warning: 20-28 value of `state("hi")` did not converge
// Hint: 20-28 the following values were observed:\n- run 1: `0`\n- run 2: `5`\n- run 3: `9`\n- run 4: `12`\n- run 5: `14`\n- final: `15`
// Hint: 20-28 see https://typst.app/help/state-convergence for help
#context hi.update(hi.get() + 6)

#let s = state("s", 1)

// Warning: 19-28 value of `state("s")` did not converge
// Hint: 19-28 the following values were observed:\n- run 1: `1`\n- run 2: `2`\n- run 3: `3`\n- run 4: `4`\n- run 5: `5`\n- final: `6`
// Hint: 19-28 see https://typst.app/help/state-convergence for help
#context s.update(s.final() + 1)

--- convergence-state-errored paged empty ---
// Warning: document did not converge within five attempts
// Hint: see 1 additional warning for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
#let s = state("s")
#switch(n => s.update(if n == 5 { _ => panic() } else { "ok" }))

// Warning: 16-23 value of `state("s")` did not converge
// Hint: 16-23 the following values were observed:\n- run 1: `none`\n- run 2: `"ok"`\n- run 3: `"ok"`\n- run 4: `"ok"`\n- run 5: `"ok"`\n- final: (errored)
// Hint: 16-23 see https://typst.app/help/state-convergence for help
#context { _ = s.get() }

--- convergence-counter paged empty ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#let c = counter("hi")

// Warning: 16-27 value of `counter("hi")` did not converge
// Hint: 16-27 the following values were observed:\n- run 1: 0\n- run 2: 0, 1\n- run 3: 0, 1, 3\n- run 4: 0, 1, 3, 7\n- run 5: 0, 1, 3, 7, 15\n- final: 0, 1, 3, 7, 15, 31
#context { _ = c.at(<end>) }

#context c.update({
  // Warning: 11-20 value of `counter("hi")` did not converge
  // Hint: 11-20 the following values were observed:\n- run 1: 0\n- run 2: 0, 1\n- run 3: 0, 1, 3\n- run 4: 0, 1, 3, 7\n- run 5: 0, 1, 3, 7, 15\n- final: 0, 1, 3, 7, 15, 31
  let v = c.final()
  v + (1 + v.last() * 2,)
})

#metadata(none) <end>

--- converge-bibliography-1 paged ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
// Warning: 26-63 citations and bibliographies did not stabilize
// Error: 26-63 bibliography could not be located
// Hint: 26-63 this bibliography is not stably present in the document
// Hint: 26-63 this can be caused by measurement or introspection
#switch(n => if n >= 5 { bibliography("/assets/bib/works.bib") })

// Error: 1-8 label `<netwok>` does not exist in the document
// Warning: 1-8 number of bibliography elements did not stabilize
// Hint: 1-8 the following numbers of elements were observed:\n- run 1: 0\n- run 2: 0\n- run 3: 0\n- run 4: 0\n- run 5: 0\n- final: 1
@netwok

--- converge-bibliography-2 paged ---
// Warning: document did not converge within five attempts
// Hint: see 2 additional warnings for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch

// Warning: 26-63 citations and bibliographies did not stabilize
#switch(n => if n >= 4 { bibliography("/assets/bib/works.bib") })

// Error: 1-8 citation could not be located
// Hint: 1-8 this citation is not stably present in the document
// Warning: 1-8 citations and bibliographies did not stabilize
// Hint: 1-8 this can be caused by measurement or introspection
@netwok

--- convergence-measure paged empty ---
// Warning: document did not converge within five attempts
// Hint: see 1 additional warning for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
#switch(n => {
  // Hint: 15-22 the closest match for this element did not stabilize
  let body = [= Hello]

  // Warning: 7-20 a measured element did not stabilize
  // Hint: 7-20 measurement tries to resolve introspections by finding the closest matching elements in the real document
  _ = measure(body)
  if n == 4 {
    body
  }
})

--- convergence-html-id html ---
// Warning: document did not converge within five attempts
// Hint: see 1 additional warning for more details
// Hint: see https://typst.app/help/convergence for help
#import "switch.typ": switch
#switch(n => calc.min(n, 4) * [= Heading <a>])

// Error: 10-55 failed to determine link anchor
// Warning: 10-55 link anchor assigned to the destination heading did not stabilize
// Hint: 10-55 the following anchors were observed:\n- run 1: (no anchor)\n- run 2: (no anchor)\n- run 3: (no anchor)\n- run 4: (no anchor)\n- run 5: (no anchor)\n- final: a-1
#context link(query(heading).last().location())[Hello]

--- convergence-state-converged-but-not-query paged empty ---
// In this example, the "high-level" state introspection yielded the same
// value in iteration 4 and 5, but the "low-level" state query yielded a
// different sequence. It also converged, but we don't know that until one
// iteration later.
#import "switch.typ": switch
#let s = state("a", none)
#switch(n => if n == 5 { s.update(none) })
#context s.get()
//@@ file tests/suite/introspection_counter.typ
// Test counters.

--- counter-basic-1 paged ---
// Count with string key.
#let mine = counter("mine!")

Final: #context mine.final().at(0) \
#mine.step()
First: #context mine.display() \
#mine.update(7)
#context mine.display("1 of 1", both: true) \
#mine.step()
#mine.step()
Second: #context mine.display("I")
#mine.update(n => n * 2)
#mine.step()

--- counter-basic-2 paged empty ---
// Test `counter`.
#let c = counter("heading")
#c.update(2)
#c.update(n => n + 2)
#context test(c.get(), (4,))
#c.update(n => n - 3)
#context test(c.at(here()), (1,))

--- counter-label paged ---
// Count labels.
#let label = <heya>
#let count = context counter(label).display()
#let elem(it) = [#box(it) #label]

#elem[hey, there!] #count \
#elem[more here!] #count

--- counter-heading paged ---
// Count headings.
#set heading(numbering: "1.a.")
#show heading: set text(10pt)
#counter(heading).step()

= Alpha
In #context counter(heading).display()
== Beta

#set heading(numbering: none)
= Gamma
#heading(numbering: "I.")[Delta]

At Beta, it was #context {
  let it = query(heading).find(it => it.body == [Beta])
  counter(heading).display(at: it.location())
}

--- counter-page paged ---
#set page(height: 50pt, margin: (bottom: 20pt, rest: 10pt))
#lines(4)
#set page(numbering: "(i)")
#lines(2)
#pagebreak()
#set page(numbering: "1 / 1")
#counter(page).update(1)
#lines(7)

--- counter-page-footer-before-set-page paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
A
#pagebreak()
#counter(page).update(5)
#set page(fill: aqua)
B

--- counter-page-header-before-set-page paged ---
#set page(numbering: "1", number-align: top + center, margin: (top: 20pt))
A
#counter(page).update(4)
#set page(fill: aqua)
B

--- counter-page-between-pages paged ---
// The update happens conceptually between the pages.
#set page(numbering: "1", margin: (bottom: 20pt))
A
#pagebreak()
#counter(page).update(5)
#set page(number-align: top + center, margin: (top: 20pt, bottom: 10pt))
B

--- counter-page-header-only-update paged ---
// Header should not be affected by default.
// To affect it, put the counter update before the `set page`.
#set page(
  numbering: "1",
  number-align: top + center,
  margin: (top: 20pt),
)

#counter(page).update(5)

--- counter-page-footer-only-update paged ---
// Footer should be affected by default.
#set page(numbering: "1 / 1", margin: (bottom: 20pt))
#counter(page).update(5)

--- counter-page-bundle bundle ---
#set page(
  numbering: "1/1",
  number-align: center,
  header: align(center, context counter(page).display("1/1", both: true)),
)

#document("a.pdf")[
  #context test(counter(page).get(), (1,))
  #context test(counter(page).final(), (5,))
  #pagebreak()
  #pagebreak()
  #context test(counter(page).get(), (3,))
  = A <a>
  #pagebreak()
  #pagebreak()
  #context test(counter(page).final(), (5,))
]

#document("b.pdf")[
  #context test(counter(page).get(), (1,))
  #context test(counter(page).final(), (7,))
  #pagebreak()
  B1 <b1>
  #counter(page).update(4)
  B2 <b2>
  #counter(page).step()
  #context test(counter(page).get(), (5,))
  #pagebreak()
  #pagebreak()
  #context test(counter(page).final(), (7,))
]

// We might want to forbid this, see `counter-page-html`.
#document("c.html")[
  #context test(counter(page).get(), (1,))
  #context test(counter(page).final(), (1,))
]

// Can access the page counter at a location even from the top-level bundle.
#context {
  test(counter(page).at(<a>), (3,))
  test(counter(page).at(<b1>), (2,))
  test(counter(page).at(<b2>), (4,))
}

--- counter-page-html html empty ---
// We might want to forbid using the page counter in HTML.
#context test(counter(page).get(), (1,))
#counter(page).step()
#context test(counter(page).get(), (2,))

--- counter-page-bundle-top-level bundle empty ---
// We might want to forbid using the page counter at the top level.
#context test(counter(page).get(), (1,))
#counter(page).step()
#context test(counter(page).get(), (2,))

--- counter-display-at paged ---
// Test displaying counter at a given location.
#set heading(numbering: "1.1")

= One
#figure(
  numbering: (..nums) => numbering(
    "1.a",
    ..((counter(heading).get().first(),) + nums.pos()),
  ),
  caption: [A blah]
)[BLAH] <blah>

= Two
#context [
  #let fig = query(<blah>).first()
  // Displaying at the figure's location is correct.
  #fig.counter.display(at: fig.location()) \
  // The manual version does not provide the correct context for resolving the
  // heading counter.
  #numbering(fig.numbering, ..fig.counter.at(fig.location())) \
  // Displaying with the numbering, but at the current location works, but does
  // not give a useful result.
  #fig.counter.display(fig.numbering) \
]

--- counter-display-matching-numbering-page paged ---
// Counter display should use the page numbering at the location.
#set page(numbering: "(i)", margin: (bottom: 20pt))
#metadata(none) <first>
Second page:
#context counter(page).display(at: <second>)

#set page(
  numbering: "A",
  footer: align(center, {
    "Page: "
    context counter(page).display()
  }),
)
#metadata(none) <second>
First page:
#context counter(page).display(at: <first>)

--- counter-display-matching-numbering-basic paged ---
// Test that `counter(heading).display()` just works: It takes care of
// using the correct location and numbering.
#show heading: it => block(counter(heading).display() + [ ] + it.body)
#heading(numbering: "1.")[One]
#heading(numbering: "A.")[Two]

--- counter-display-matching-numbering-full paged ---
// Tests that the determination of the matching numbering is comprehensive for
// all supported elements.

// This should be overridden by the element's numbering.
#set heading(numbering: "(i)")
#set math.equation(block: true)

#let funcs = (heading, figure, math.equation, footnote)
#show selector.or(..funcs): it => counter(it.func()).display()
#for f in funcs {
  block(f(numbering: "a)")[])
}

--- counter-display-matching-numbering-wrong paged ---
// Test that we don't pick up a numbering unrelated to the counted element.
#set heading(numbering: "A)")
#set math.equation(numbering: "1.")
= Hello
$ 1 + 2 $ <eq>
#context counter(heading).display(at: <eq>)

--- counter-display-matching-numbering-fallback paged ---
// Test fallback case where the matching numbering is determined from the style
// chain instead of the element. Because there is no heading element at `<at>`,
// we fall back to the style that is current at the counter display, not the
// style that was current at the location. Ideally, we'd also handle this, but
// its not trivial and this should be very rare.
#set heading(numbering: "A)")
= Hello
#metadata(none) <at>
#set heading(numbering: "I)")
#context counter(heading).display(at: <at>)

--- counter-figure paged ---
// Count figures.
#figure(numbering: "A", caption: [Four 'A's], kind: image, supplement: "Figure")[_AAAA!_]
#figure(numbering: none, caption: [Four 'B's], kind: image, supplement: "Figure")[_BBBB!_]
#figure(caption: [Four 'C's], kind: image, supplement: "Figure")[_CCCC!_]
#counter(figure.where(kind: image)).update(n => n + 3)
#figure(caption: [Four 'D's], kind: image, supplement: "Figure")[_DDDD!_]

--- counter-at-no-context eval ---
// Test `counter.at` outside of context.
// Error: 2-28 can only be used when context is known
// Hint: 2-28 try wrapping this in a `context` expression
// Hint: 2-28 the `context` expression should wrap everything that depends on this function
#counter("key").at(<label>)

--- counter-update-panic paged trace ---
// Error: 22-29 panicked
#let page-count(n) = panic()

// Trace: 2-34 (1) while calling `page-count`
#state("page").update(page-count)

// Trace: 1:10-3:2 (3) while evaluating contextual content
#context {
  // Trace: 3-24 (2) while calling `final`
  state("page").final()
}

--- issue-2480-counter-reset paged ---
#let q = counter("question")
#let step-show =  q.step() + context q.display("1")
#let g = grid(step-show, step-show, gutter: 2pt)

#g
#pagebreak()
#step-show
#q.update(10)
#g

--- issue-2480-counter-reset-2 paged ---
#set block(spacing: 3pt)
#let c = counter("c")
#let foo() = context {
  c.step()
  c.display("1")
  str(c.get().first())
}

#foo()
#block(foo())
#foo()
#foo()
#block(foo())
#block(foo())
#foo()

--- issue-4626-counter-depth-skip paged empty ---
// When we step and skip a level, the levels should be filled with zeros, not
// with ones.
#let c = counter("c")
#context test(c.get(), (0,))
#c.step(level: 4)
#context test(c.get(), (0, 0, 0, 1))
#c.step(level: 1)
#context test(c.get(), (1,))
#c.step(level: 3)
#context test(c.get(), (1, 0, 1))

--- counter-huge paged empty ---
// Test values greater than 32-bits
#let c = counter("c")
#c.update(100000000001)
#context test(c.get(), (100000000001,))
#c.step()
#context test(c.get(), (100000000002,))
#c.update(n => n + 2)
#context test(c.get(), (100000000004,))

--- counter-rtl paged ---
#set page(width: auto)
#let c = counter("c")
#let s = context c.display() + c.step()
#let tree = [درخت]
#let line = [A #s B #tree #s #tree #s #tree C #s D #s]
#line \
#line
//@@ file tests/suite/introspection_here.typ
--- here-position paged empty ---
// Test `context` + `here`.
#context test(here().position().y, 10pt)
//@@ file tests/suite/introspection_locate.typ
--- locate-position paged ---
// Test `locate`.
#v(10pt)
= Introduction <intro>
#context test(locate(<intro>).position().y, 20pt)

--- locate-position-trailing-tag paged ---
// Test locating the position of a tag with no following content.
#context test(here().position().y, 10pt)
#box[]
#v(10pt)
#context test(here().position().y, 20pt)

--- locate-start-of-par paged ---
#metadata(none)<a>A#metadata(none)<b>B

// The first metadata has its end tag before the paragraph, so it does not
// become part of the paragraph and thus its Y position is determined by the
// flow.
#context assert(
  locate(<a>).position().y < locate(<b>).position().y
)

// The first footnote becomes part of the paragraph. Thus, its Y position is
// determined by inline layout.
#footnote[c]<c>C#footnote[d]<d>D

#context test(
  locate(<c>).position().y,
  locate(<d>).position().y,
)

--- locate-missing-label paged ---
// Error: 10-25 label `<intro>` does not exist in the document
#context locate(<intro>)

--- locate-duplicate-label paged ---
= Introduction <intro>
= Introduction <intro>

// Error: 10-25 label `<intro>` occurs multiple times in the document
#context locate(<intro>)

--- locate-element-selector paged ---
#v(10pt)
= Introduction <intro>
#context test(locate(heading).position().y, 20pt)

--- locate-element-selector-no-match paged ---
// Error: 10-25 selector does not match any element
#context locate(heading)

--- locate-element-selector-multiple-matches paged ---
= Introduction <intro>
= Introduction <intro>

// Error: 10-25 selector matches multiple elements
#context locate(heading)

--- locate-between-pages paged ---
// Test locating tags that are before or between pages.
#set page(height: 30pt)
#context [
  // Before the first page.
  // (= at the very start of the first page, before the header)
  #test(locate(<a>).position(), (page: 1, x: 0pt, y: 0pt))

  // On the first page.
  #test(locate(<b>).position(), (page: 1, x: 10pt, y: 10pt))

  // Between the two pages.
  // (= at the very start of the first page, before the header)
  #test(locate(<c>).position(), (page: 2, x: 0pt, y: 0pt))

  // After the last page.
  // (= at the very end of the last page, after the footer)
  #test(locate(<d>).position(), (page: 2, x: 0pt, y: 30pt))
  #test(locate(<e>).position(), (page: 2, x: 0pt, y: 30pt))
]

#metadata(none) <a>
#pagebreak(weak: true)
#metadata(none) <b>
A
#pagebreak()
#metadata(none) <c>
#pagebreak(weak: true)
B
#pagebreak(weak: true)
#metadata(none) <d>
#pagebreak(weak: true)
#metadata(none) <e>

--- locate-migrated-breakable paged ---
// Ensure that when a breakable element fully migrates to the next page without
// orphan frames, its position correctly reflects that.
#set page(height: 40pt)
A
#block[B]<b>

#context test(
  locate(<b>).position(),
  (page: 2, x: 10pt, y: 10pt),
)

--- locate-html html empty ---
#metadata(none)
// Outside of a frame, there are no coordinates. See `html-frame-position`.
#context test(
  locate(metadata).position(),
  none
)

--- locate-bundle-top-level bundle ---
#metadata(none)
#document("index.html")[
  // The metadata is not part of any document, so it has no page/position.
  #context test(locate(metadata).page(), none)
  #context test(locate(metadata).position(), none)
]

--- locate-bundle-in-pdf bundle ---
#context {
  let loc = locate(<a>)
  test(loc.page(), 2)
  test(loc.position(), (page: 2, x: 10pt, y: 10pt))
}

#document("main.pdf")[
  #pagebreak()
  = Heading <a>
]

--- locate-bundle-duplicates bundle ---
// Ensures that elements that are queried and then placed in another document
// are still only observed once by queries. This tests how document
// introspectors are combined into a bundle introspector.
#context test(query(heading).len(), 1)
#document("a.html", context query(<b>).first())
#document("b.html")[= B <b>]

--- issue-4029-locate-after-spacing paged ---
#set page(margin: 10pt)
#show heading: it => v(40pt) + it

= Introduction
#context test(
  locate(heading).position(),
  (page: 1, x: 10pt, y: 50pt),
)


--- issue-4029-locate-after-pagebreak paged ---
#set page(margin: 10pt)
#show heading: it => pagebreak() + it

= Introduction
#context test(
  locate(heading).position(),
  (page: 2, x: 10pt, y: 10pt),
)

--- issue-4029-locate-after-par-and-pagebreak paged ---
// Ensure that the heading's tag isn't stuck at the end of the paragraph.
#set page(margin: 10pt)
Par
#show heading: it => pagebreak() + it
= Introduction
#context test(locate(heading).page(), 2)

--- issue-1886-locate-after-metadata paged ---
#show heading: it => {
  metadata(it.label)
  pagebreak(weak: true, to: "odd")
  it
}

Hi
= Hello <hello>
= World <world>

// The metadata's position does not migrate to the next page, but the heading's
// does.
#context {
  test(locate(metadata.where(value: <hello>)).page(), 1)
  test(locate(<hello>).page(), 3)
  test(locate(metadata.where(value: <world>)).page(), 3)
  test(locate(<world>).page(), 5)
}

--- issue-1833-locate-place paged ---
#set page(height: 60pt)
#context {
  place(right + bottom, rect())
  test(here().position(), (page: 1, x: 10pt, y: 10pt))
}
//@@ file tests/suite/introspection_query.typ
// Test creating a header with the query function.

--- query-here paged empty ---
// Test that `here()` yields the context element's location.
#context test(query(here()).first().func(), (context none).func())

--- query-running-header paged ---
#set page(
  paper: "a8",
  margin: (y: 1cm, x: 0.5cm),
  header: context {
    smallcaps[Typst Academy]
    h(1fr)
    let after = query(selector(heading).after(here()))
    let before = query(selector(heading).before(here()))
    let elem = if before.len() != 0 {
      before.last()
    } else if after.len() != 0 {
      after.first()
    }
    emph(elem.body)
  }
)

#outline()

= Introduction
#lines(1)

= Background
#lines(2)

= Approach

--- query-list-of-figures paged ---
#set page(
  paper: "a8",
  numbering: "1 / 1",
  margin: (bottom: 1cm, rest: 0.5cm),
)

#set figure(numbering: "I")
#show figure: set image(width: 80%)

= List of Figures
#context {
  let elements = query(selector(figure).after(here()))
  for it in elements [
    Figure
    #counter(figure).display(at: it.location()):
    #it.caption.body
    #box(width: 1fr, repeat[.])
    #counter(page).at(it.location()).first() \
  ]
}

#figure(
  image("/assets/images/cylinder.svg", width: 50%),
  caption: [Cylinder],
)

#figure(
  rect[Just some stand-in text],
  kind: image,
  supplement: "Figure",
  caption: [Stand-in text],
)

#figure(
  image("/assets/images/tetrahedron.svg", width: 50%),
  caption: [Tetrahedron],
)

--- query-complex paged ---
= A
== B
#figure([Cat], kind: "cat", supplement: [Other])
#heading(level: 3, outlined: false)[D]
= E <first>
#figure([Frog], kind: "frog", supplement: none)
#figure([Giraffe], kind: "giraffe", supplement: none) <second>
#figure([GiraffeCat], kind: "cat", supplement: [Other]) <second>
= H
#figure([Iguana], kind: "iguana", supplement: none)
== I

#let test-selector(selector, ref) = context {
  test(query(selector).map(e => e.body), ref)
}

// Test `or`.
#test-selector(
  heading.where(level: 1).or(heading.where(level: 3)),
  ([A], [D], [E], [H]),
)

#test-selector(
  selector(heading).and(heading.where(outlined: false)),
  ([D],)
)

#test-selector(
  heading.where(level: 1).or(
    heading.where(level: 3),
    figure.where(kind: "frog"),
  ),
  ([A], [D], [E], [Frog], [H]),
)

#test-selector(
  heading.where(level: 1).or(
    heading.where(level: 2),
    figure.where(kind: "frog"),
    figure.where(kind: "cat"),
  ),
  ([A], [B], [Cat], [E], [Frog], [GiraffeCat], [H], [I]),
)

#test-selector(
  figure.where(kind: "dog").or(heading.where(level: 3)),
  ([D],),
)

#test-selector(
  figure.where(kind: "dog").or(figure.where(kind: "fish")),
  (),
)

// Test `or` duplicates removal.
#test-selector(
  heading.where(level: 1).or(heading.where(level: 1)),
  ([A], [E], [H]),
)

// Test `and`.
#test-selector(
  figure.where(kind: "cat").and(figure.where(kind: "frog")),
  (),
)

// Test `or` with `before`/`after`
#test-selector(
  selector(heading)
    .before(<first>)
    .or(selector(figure).before(<first>)),
  ([A], [B], [Cat], [D], [E]),
)

#test-selector(
  heading.where(level: 2)
    .after(<first>)
    .or(selector(figure).after(<first>)),
  ([Frog], [Giraffe], [GiraffeCat], [Iguana], [I]),
)

// Test `and` with `after`
#test-selector(
   figure.where(kind: "cat")
    .and(figure.where(supplement: [Other]))
    .after(<first>),
   ([GiraffeCat],),
)

// Test `and` (with nested `or`)
#test-selector(
  heading.where(level: 2)
    .or(heading.where(level: 3))
    .and(heading.where(level: 2).or(heading.where(level: 1))),
  ([B], [I]),
)

#test-selector(
  heading.where(level: 2)
    .or(heading.where(level: 3), heading.where(level:1))
    .and(
      heading.where(level: 2).or(heading.where(level: 1)),
      heading.where(level: 3).or(heading.where(level: 1)),
    ),
  ([A], [E], [H]),
)

// Test `and` with `or` and `before`/`after`
#test-selector(
  heading.where(level: 1).before(<first>)
    .or(heading.where(level: 3).before(<first>))
    .and(
      heading.where(level: 1).before(<first>)
        .or(heading.where(level: 2).before(<first>))
    ),
  ([A], [E]),
)

#test-selector(
  heading.where(level: 1).before(<first>, inclusive: false)
    .or(selector(figure).after(<first>))
    .and(figure.where(kind: "iguana").or(
      figure.where(kind: "frog"),
      figure.where(kind: "cat"),
      heading.where(level: 1).after(<first>),
    )),
  ([Frog], [GiraffeCat], [Iguana])
)

--- query-within paged html bundle ---
// We also want to test in bundle mode to ensure the inner introspector
// correctly forwards the stuff.
#show: it => context if target() == "bundle" {
  document("main.pdf", it)
} else {
  it
}

#let test-selector(selector, ref) = context {
  test(query(selector).map(e => e.body), ref)
}

= #emph[Hi *there*]

What's *up* with *you?*

#figure([Empty], caption: [A *nice* *rect*])

// Test that the within query gracefully handles a case where an insertion
// immediately precedes the end tag.
#quote(footnote[_Hello_])

#context [
  #let loc = here()
  *Local* bold *text*
  #test-selector(
    selector(strong).within(loc),
    ([Local], [text]),
  )
]

#test-selector(
  selector(strong).within(par),
  ([up], [you?], [Local], [text]),
)

#test-selector(
  selector(strong).within(selector.or(heading, emph, figure)),
  ([there], [nice], [rect]),
)

#test-selector(
  selector(strong).within(emph).within(heading),
  ([there],),
)

#test-selector(
  selector(strong).within(heading).within(emph),
  ([there],),
)

#test-selector(
  selector(strong).within(selector(heading).within(emph)),
  (),
)

#test-selector(
  selector.within(emph, quote),
  ([Hello],),
)

--- query-within-document bundle ---
#let test-selector(selector, ref) = context {
  test(query(selector).map(e => e.body), ref)
}

#document("a.html")[
  = 1
  #table[
    = 2
  ][
    = 3
  ]
] <a>

#document("b.html")[
  = 4
  = 5
] <b>

#test-selector(selector.within(heading, table), ([2], [3]))
#test-selector(selector.within(heading, <a>), ([1], [2], [3]))
#test-selector(selector.within(heading, <b>), ([4], [5]))

--- query-within-inverted paged empty ---
// Test a case where the ancestor is fully contained in one of the children.
#let m(l) = [#metadata(none)#l]
#strong({
  m(<a>)
  m(<b>)
  m(<c>)
})
#context test(query(selector.within(strong, <b>)), ())

--- query-bundle-logical-order bundle ---
#metadata(1)
#document("hi.html")[
  #metadata("a")
  HTML
]
#metadata(2)
#asset("data.json", "data")
#metadata(3)
#document("hi.pdf")[
  #metadata("b")
  PDF
]
#metadata(4)

// Test logical order across documents.
#context {
  test(query(metadata).map(v => v.value), (1, "a", 2, 3, "b", 4))
}

// Test querying documents and assets.
#context {
  test(query(document).map(v => v.path), ("/hi.html", "/hi.pdf"))
  test(query(asset).map(v => v.path), ("/data.json",))
  test(
    query(selector.or(document, asset)).map(v => v.path),
    ("/hi.html", "/data.json", "/hi.pdf"),
  )
  test(
    query(selector(document).after(metadata.where(value: 3))).map(v => v.path),
    ("/hi.pdf",),
  )
}

--- query-bundle-logical-order-around-html bundle ---
#metadata(1)
#document("hi.html")[
  // Test tags around the HTML root element.
  #metadata("a")
  #html.html[
    #metadata("b")
    #html.body[Hello World!]
    #metadata("c")
  ]
  #metadata("d")
]
#metadata(2)

#context test(
  query(metadata).map(v => v.value),
  (1, "a", "b", "c", "d", 2)
)

--- issue-3726-query-show-set paged ---
// Test that show rules apply to queried elements, i.e. that the content
// returned from `query` isn't yet marked as prepared.
#set heading(numbering: "1.")
#show heading: underline
= Hi

#set heading(numbering: "I.")
#show heading: set text(blue)
#show heading: highlight.with(fill: aqua.lighten(50%))
= Bye

// New show rules apply to this, but its location and the materialized fields
// from the original are retained.
#context query(heading).join()

--- query-quote paged ---
// Test quoting a query.

#quote[ABC] & #quote[EFG]

#context query(selector(quote).before(here())).first()

#quote(block: true)[HIJ]
#quote(block: true)[KLM]

#context query(selector(quote).before(here())).last()

#quote[NOP] <nop>

#context query(<nop>).first()

--- issue-5117-query-order-place paged empty ---
#let t(expected) = context {
  let elems = query(selector(metadata).after(here()))
  let val = elems.first().value
  test(val, expected)
}

#{
  t("a")
  place(metadata("a"))
}

#{
  t("b")
  block(height: 1fr, metadata("b"))
}
//@@ file tests/suite/introspection_state.typ
// Test state.

--- state-basic paged ---
#let s = state("hey", "a")
#let double(it) = 2 * it

#s.update(double)
#s.update(double)
$ 2 + 3 $
#s.update(double)

Is: #context s.get(),
Was: #context {
  let it = query(math.equation).first()
  s.at(it.location())
}.

--- state-multiple-calls-same-key paged ---
// Try same key with different initial value.
#context state("key", 2).get()
#state("key").update(x => x + 1)
#context state("key", 2).get()
#context state("key", 3).get()
#state("key").update(x => x + 1)
#context state("key", 2).get()

--- state-nested paged ---
#set page(width: 200pt)
#set text(8pt)

#let ls = state("lorem", lorem(30).split(" "))
#let loremum(count) = {
  context ls.get().slice(0, count).join(".").trim() + "."
  ls.update(list => list.slice(count))
}

#let fs = state("fader", red)
#let trait(title) = block[
  #context text(fill: fs.get())[
    *#title:* #loremum(1)
  ]
  #fs.update(color => color.lighten(30%))
]

#trait[Boldness]
#trait[Adventure]
#trait[Fear]
#trait[Anger]

--- state-at-no-context eval ---
// Test `state.at` outside of context.
// Error: 2-26 can only be used when context is known
// Hint: 2-26 try wrapping this in a `context` expression
// Hint: 2-26 the `context` expression should wrap everything that depends on this function
#state("key").at(<label>)

--- state-update-panic paged trace ---
// Error: 22-29 panicked
#let page-count(n) = panic()

// Trace: 2-34 (1) while calling `page-count`
#state("page").update(page-count)

// Trace: 1:10-3:2 (3) while evaluating contextual content
#context {
  // Trace: 3-24 (2) while calling `final`
  state("page").final()
}
//@@ file tests/suite/introspection_switch.typ
// SKIP

// Calls `f` with the current layout iteration and displays the return
// value of `f`.
#let switch(f) = context {
  // The `here()` trick is just to produce a counter unique to this context
  // block.
  let c = counter(metadata.where(value: here()))
  let i = c.final().first()
  let n = i + 1
  c.update(if n < 5 { i + 1 } else { i })
  context f(n)
}
//@@ file tests/suite/introspection_tags.typ
--- tags-grouping paged empty ---
// Test how grouping rules handle tags at their edges. To observe this scenario,
// we have a link at the start or end of a paragraph and see whether the two
// nest correctly.
//
// The tag nesting check is done as a custom check as, unfortunately, there isn't
// really a way to test end tags from Typst code.
//
// On top that we are checking that trailing start/end combos tags after the
// grouping, but before a tag that is matched in the group are also kept in the
// grouping. This we can check from within Typst code via show rules with
// metadata, which is done below.

// Hide everything ... we don't need a reference image.
#set text(size: 0pt)
#show: hide
#show ref: [Ref]

#let case(body, output) = context {
  // Get a unique key for the case.
  let key = here()
  let tagged(s, it) = {
    metadata((key, "<" + s + ">"))
    it
    metadata((key, "</" + s + ">"))
  }

  // Note: This only works for locatable elements because otherwise the
  // metadata tags won't be sandwiched by other tags and not forced into the
  // paragraph grouping.
  show par: tagged.with("p")
  show link: tagged.with("a")
  show ref: tagged.with("ref")
  body

  context test(
    // Finds only metadata keyed by `key`, which is unique for this case.
    query(metadata)
      .filter(e => e.value.first() == key)
      .map(e => e.value.last())
      .join(),
    output
  )
}

// Because `par` is not currently locatable, we need another way to ensure
// tags are generated around it. This can be removed once par is locatable.
#show par: quote
#show quote: it => it.body

// Both link and ref are contained in the paragraph.
#case(
  [@ref #link("A")[A]],
  "<p><ref></ref><a></a></p>"
)

// When there's a trailing space, that's okay.
#case(
  [@ref #link("A")[A ]],
  "<p><ref></ref><a></a></p>"
)

// Both link and ref are contained in the paragraph.
#case(
  [#link("A")[A] @ref],
  "<p><a></a><ref></ref></p>"
)

// When there's a leading space, that's okay.
#case(
  [#link("A")[ A] @ref],
  "<p><a></a><ref></ref></p>"
)

// When there's only a link, it will surround the paragraph.
#case(
  link("A")[A],
  "<p><a></a></p>"
)

--- tags-textual paged ---
// Ensure that tags and spaces aren't reordered in textual grouping.
A#metadata(none)<a> #metadata(none)<b>#box[B]

#context assert(
  locate(<a>).position().x + 1pt < locate(<b>).position().x
)
//@@ file tests/suite/layout_align.typ
// Test alignment.

--- align-right paged ---
// Test ragged-left.
#set align(right)
To the right! Where the sunlight peeks behind the mountain.

--- align-in-stack paged ---
#set page(height: 100pt)
#stack(dir: ltr,
  align(left, square(size: 15pt, fill: eastern)),
  align(center, square(size: 20pt, fill: eastern)),
  align(right, square(size: 15pt, fill: eastern)),
)
#align(center + horizon, rect(fill: eastern, height: 10pt))
#align(bottom, stack(
  align(center, rect(fill: conifer, height: 10pt)),
  rect(fill: forest, height: 10pt, width: 100%),
))

--- align-center-in-flow paged ---
// Test that multiple paragraphs in subflow also respect alignment.
#align(center)[
  Lorem Ipsum

  Dolor
]

--- align-start-and-end paged ---
// Test start and end alignment.
#rotate(-30deg, origin: end + horizon)[Hello]

#set text(lang: "de")
#align(start)[Start]
#align(end)[Ende]

#set text(lang: "ar", font: "Noto Sans Arabic")
#align(start)[يبدأ]
#align(end)[نهاية]

--- alignment-fields-x eval ---
// Test 2d alignment 'horizontal' field.
#test((start + top).x, start)
#test((end + top).x, end)
#test((left + top).x, left)
#test((right + top).x, right)
#test((center + top).x, center)
#test((start + bottom).x, start)
#test((end + bottom).x, end)
#test((left + bottom).x, left)
#test((right + bottom).x, right)
#test((center + bottom).x, center)
#test((start + horizon).x, start)
#test((end + horizon).x, end)
#test((left + horizon).x, left)
#test((right + horizon).x, right)
#test((center + horizon).x, center)
#test((top + start).x, start)
#test((bottom + end).x, end)
#test((horizon + center).x, center)

--- alignment-fields-y eval ---
// Test 2d alignment 'vertical' field.
#test((start + top).y, top)
#test((end + top).y, top)
#test((left + top).y, top)
#test((right + top).y, top)
#test((center + top).y, top)
#test((start + bottom).y, bottom)
#test((end + bottom).y, bottom)
#test((left + bottom).y, bottom)
#test((right + bottom).y, bottom)
#test((center + bottom).y, bottom)
#test((start + horizon).y, horizon)
#test((end + horizon).y, horizon)
#test((left + horizon).y, horizon)
#test((right + horizon).y, horizon)
#test((center + horizon).y, horizon)
#test((top + start).y, top)
#test((bottom + end).y, bottom)
#test((horizon + center).y, horizon)

--- alignment-type eval ---
#test(type(center), alignment)
#test(type(horizon), alignment)
#test(type(center + horizon), alignment)

--- alignment-axis eval ---
// Test alignment methods.
#test(start.axis(), "horizontal")
#test(end.axis(), "horizontal")
#test(left.axis(), "horizontal")
#test(right.axis(), "horizontal")
#test(center.axis(), "horizontal")
#test(top.axis(), "vertical")
#test(bottom.axis(), "vertical")
#test(horizon.axis(), "vertical")

--- alignment-inv eval ---
#test(start.inv(), end)
#test(end.inv(), start)
#test(left.inv(), right)
#test(right.inv(), left)
#test(center.inv(), center)
#test(top.inv(), bottom)
#test(bottom.inv(), top)
#test(horizon.inv(), horizon)
#test((start + top).inv(), (end + bottom))
#test((end + top).inv(), (start + bottom))
#test((left + top).inv(), (right + bottom))
#test((right + top).inv(), (left + bottom))
#test((center + top).inv(), (center + bottom))
#test((start + bottom).inv(), (end + top))
#test((end + bottom).inv(), (start + top))
#test((left + bottom).inv(), (right + top))
#test((right + bottom).inv(), (left + top))
#test((center + bottom).inv(), (center + top))
#test((start + horizon).inv(), (end + horizon))
#test((end + horizon).inv(), (start + horizon))
#test((left + horizon).inv(), (right + horizon))
#test((right + horizon).inv(), (left + horizon))
#test((center + horizon).inv(), (center + horizon))
#test((top + start).inv(), (end + bottom))
#test((bottom + end).inv(), (start + top))
#test((horizon + center).inv(), (center + horizon))

--- alignment-add-two-horizontal eval ---
// Error: 8-22 cannot add two horizontal alignments
#align(center + right, [A])

--- alignment-add-two-vertical eval ---
// Error: 8-20 cannot add two vertical alignments
#align(top + bottom, [A])

--- alignment-add-vertical-and-2d eval ---
// Error: 8-30 cannot add a vertical and a 2D alignment
#align(top + (bottom + right), [A])

--- issue-1398-line-align paged ---
// Test right-aligning a line and a rectangle.
#align(right, line(length: 30%))
#align(right, rect())

--- issue-2213-align-fr paged ---
// Test a mix of alignment and fr units (fr wins).
#set page(height: 80pt)
A
#v(1fr)
B
#align(bottom + right)[C]
//@@ file tests/suite/layout_angle.typ
--- angle-to-unit eval ---
// Test angle methods.
#test(1rad.rad(), 1.0)
#test(1.23rad.rad(), 1.23)
#test(0deg.rad(), 0.0)
#test(2deg.deg(), 2.0)
#test(2.94deg.deg(), 2.94)
#test(0rad.deg(), 0.0)
//@@ file tests/suite/layout_clip.typ

//@@ file tests/suite/layout_columns.typ
// Test the column layouter.

--- columns-rtl paged ---
// Test normal operation and RTL directions.
#set page(height: 3.25cm, width: 7.05cm, columns: 2)
#set text(lang: "ar", font: ("Noto Sans Arabic", "Libertinus Serif"))
#set columns(gutter: 30pt)

#box(fill: conifer, height: 8pt, width: 6pt) وتحفيز
العديد من التفاعلات الكيميائية. (DNA) من أهم الأحماض النووية التي تُشكِّل
إلى جانب كل من البروتينات والليبيدات والسكريات المتعددة
#box(fill: eastern, height: 8pt, width: 6pt)
الجزيئات الضخمة الأربعة الضرورية للحياة.

--- columns-in-fixed-size-rect paged ---
// Test the `columns` function.
#set page(width: auto)

#rect(width: 180pt, height: 100pt, inset: 8pt, columns(2, [
    A special plight has befallen our document.
    Columns in text boxes reigned down unto the soil
    to waste a year's crop of rich layouts.
    The columns at least were graciously balanced.
]))

--- columns-set-page paged ---
// Test columns for a sized page.
#set page(height: 5cm, width: 7.05cm, columns: 2)

Lorem ipsum dolor sit amet is a common blind text
and I again am in need of filling up this page
#align(bottom, rect(fill: eastern, width: 100%, height: 12pt))
#colbreak()

so I'm returning to this trusty tool of tangible terror.
Sure, it is not the most creative way of filling up
a page for a test but it does get the job done.

--- columns-in-auto-sized-rect paged ---
// Test the expansion behaviour.
#set page(height: 2.5cm, width: 7.05cm)

#rect(inset: 6pt, columns(2, [
    ABC \
    BCD
    #colbreak()
    DEF
]))

--- columns-more-with-gutter paged ---
// Test setting a column gutter and more than two columns.
#set page(height: 3.25cm, width: 7.05cm, columns: 3)
#set columns(gutter: 30pt)

#rect(width: 100%, height: 2.5cm, fill: conifer) #parbreak()
#rect(width: 100%, height: 2cm, fill: eastern) #parbreak()
#circle(fill: eastern)

--- columns-set-page-colbreak-pagebreak paged ---
// Test the `colbreak` and `pagebreak` functions.
#set page(height: 1cm, width: 7.05cm, columns: 2)

A
#colbreak()
#colbreak()
B
#pagebreak()
C
#colbreak()
D

--- columns-empty-second-column paged ---
// Test an empty second column.
#set page(width: 7.05cm, columns: 2)

#rect(width: 100%, inset: 3pt)[So there isn't anything in the second column?]

--- columns-page-width-auto paged ---
// Test columns when one of them is empty.
#set page(width: auto, columns: 3)

Arbitrary horizontal growth.

--- columns-page-height-auto paged ---
// Test columns in an infinitely high frame.
#set page(width: 7.05cm, columns: 2)

There can be as much content as you want in the left column
and the document will grow with it.

#rect(fill: conifer, width: 100%, height: 30pt)

Only an explicit #colbreak() `#colbreak()` can put content in the
second column.

--- columns-one paged ---
// Test a page with a single column.
#set page(height: auto, width: 7.05cm, columns: 1)

This is a normal page. Very normal.

--- columns-zero eval ---
// Test a page with zero columns.
// Error: 49-50 number must be positive
#set page(height: auto, width: 7.05cm, columns: 0)

--- columns-colbreak-after-place paged ---
// Test colbreak after only out-of-flow elements.
#set page(width: 7.05cm, columns: 2)
#place[OOF]
#colbreak()
In flow.

--- issue-columns-heading paged ---
// The well-known columns bug.
#set page(height: 70pt)

Hallo
#columns(2)[
  = A
  Text
  = B
  Text
]

--- colbreak-weak paged ---
#set page(columns: 2)
#colbreak(weak: true)
A
#colbreak(weak: true)
B

--- columns-balanced paged ---
#set page(width: 300pt)
#set par(justify: true)
#rect(inset: 0pt, outset: 0pt, stroke: yellow,
  columns(3, balanced: true, separator: line(), lorem(40))
)

--- columns-balanced-leftover-2-3 paged ---
// Test how column balancing distributes leftover lines across columns
#set page(width: 300pt)
#set text(size: 7pt)
#set par(justify: true)
#for (n, mm) in ((2, (31, 37)), (3, (29, 32, 37))) {
  for m in mm {
    rect(inset: 0pt, outset: 0pt, stroke: gray,
      columns(n, balanced: true, separator: line(), lorem(m))
    )
  }
}

--- columns-balanced-leftover-4 paged ---
// Test how column balancing distributes leftover lines across columns
#set page(width: 300pt)
#set text(size: 7pt)
#set par(justify: true)
#for (n, mm) in ((4, (26, 29, 32, 35)),) {
  for m in mm {
    rect(inset: 0pt, outset: 0pt, stroke: gray,
      columns(n, balanced: true, separator: line(), lorem(m))
    )
  }
}

--- columns-balanced-leftover-5 paged ---
// Test how column balancing distributes leftover lines across columns
#set page(width: 300pt)
#set text(size: 7pt)
#set par(justify: true)
#for (n, mm) in ((5, (21, 25, 27, 29, 31)),) {
  for m in mm {
    rect(inset: 0pt, outset: 0pt, stroke: gray,
      columns(n, balanced: true, separator: line(), lorem(m))
    )
  }
}

--- columns-balanced-leftover-6 paged ---
// Test how column balancing distributes leftover lines across columns
#set page(width: 350pt)
#set text(size: 6pt)
#set par(justify: true)
#for (n, mm) in ((6, (29, 31, 33, 35, 37, 39)),) {
  for m in mm {
    rect(inset: 0pt, outset: 0pt, stroke: gray,
      columns(n, balanced: true, separator: line(), lorem(m))
    )
  }
}

--- columns-balanced-leftover-7 paged ---
// Test how column balancing distributes leftover lines across columns
#set page(width: 350pt)
#set text(size: 6pt)
#set text(hyphenate: true)
#set par(justify: true)
#for (n, mm) in ((7, (28, 30, 31, 32, 34, 36, 38)),) {
  for m in mm {
    rect(inset: 0pt, outset: 0pt, stroke: gray,
      columns(n, balanced: true, separator: line(), lorem(m))
    )
  }
}

--- columns-balanced-widow paged ---
#set page(width: 300pt, height: 100pt, columns: 6)
#set text(size: 7pt)
#set par(justify: true)
#set columns(balanced: true, separator: line())
#lorem(68) // just enough text to trigger a widow if not balanced correctly

--- columns-balanced-pagebreak paged ---
#set page(width: 300pt, height: 90pt, margin: 5pt, columns: 3)
#set text(hyphenate: true)
#set par(justify: true)
#set columns(balanced: true, separator: line())
#lorem(35)
#pagebreak()
#lorem(18)

--- columns-balanced-figures paged ---
#set page(width: 300pt, height: 190pt, margin: 5pt, columns: 3)
#set columns(balanced: true, separator: line())
#set text(hyphenate: true)
#set par(justify: true)
#figure(rect(height: 30pt, width: 100%))
#lorem(25)#footnote[Footer]
#figure(rect(height: 30pt, width: 100%), placement: bottom)
#lorem(35)
#figure(rect(height: 15pt, width: 80%), scope: "parent", placement: bottom)

--- columns-balanced-footnotes paged ---
#set page(width: 300pt, height: 90pt, margin: 5pt, columns: 3)
#set text(hyphenate: true)
#set par(justify: true)
#set columns(balanced: true, separator: line())
#lorem(10)#footnote[Footer]
#lorem(50)#footnote[Footer]
#lorem(22)

--- columns-balanced-footnote-overflow paged ---
#set page(width: 300pt, height: 150pt, columns: 2)
#set columns(balanced: true, separator: line())
#lorem(10)
#footnote(lorem(30))
#lorem(45)

--- columns-balanced-blocks paged ---
#set page(width: 100pt, height: auto, margin: 5pt, columns: 3)
#set columns(balanced: true, separator: line())
#set block(fill: gray, spacing: 1pt)
#for i in range(10){block(width: 100%, height: 3pt * (1+i/5))}
#pagebreak()
#set block(breakable: false)
#for i in range(10){block(width: 100%, height: 3pt * (1+i/5))}

--- columns-balanced-tables paged ---
// Check that balancing is repeated when
// the height changes due to the repeated header
#set page(width: 100pt, margin: 5pt, columns: 2)
#set columns(balanced: true, separator: line())
#table(columns: 2,
  table.header([$x$], [$x^2$]),
  ..for x in range(1,11){
    ([#x], [#(x*x)])
  }
)

--- columns-balanced-lists paged ---
// Check that breakable blocks are balanced correctly via `multi` and `multi_spill`
#set page(width: 350pt, columns: 4)
#set columns(balanced: true, separator: line())
#{
  8 * [- #lorem(5)]
}

--- columns-separator paged ---
#set page(width: 200pt, height: 100pt, columns: 2)
#set text(hyphenate: true)
#set par(justify: true)
#set columns(separator: line(stroke: red + 1pt))
#lorem(30)

--- columns-separator-content paged ---
#set page(width: 200pt, height: 100pt, columns: 2)
#set text(hyphenate: true)
#set par(justify: true)
#set curve(stroke: blue)
#set columns(
  gutter: 20pt,
  separator: curve(curve.cubic((-8pt, 0%), (8pt, 50%), (0pt, 50%)))
)
#lorem(29)

--- columns-separator-rtl paged ---
#set page(columns: 2)
#set columns(separator: line(), gutter: 20pt)
#set text(dir: rtl)
#rect(width: 100%)[1]
#colbreak()
#rect(width: 100%)[2]
//@@ file tests/suite/layout_container.typ
// Test the `box` and `block` containers.

--- box paged ---
// Test box in paragraph.
A #box[B \ C] D.

// Test box with height.
Spaced \
#box(height: 0.5cm) \
Apart

--- block-sizing paged ---
// Test block sizing.
#set page(height: 120pt)
#set block(spacing: 0pt)
#block(width: 90pt, height: 80pt, fill: red)[
  #block(width: 60%, height: 60%, fill: green)
  #block(width: 50%, height: 60%, fill: blue)
]

--- box-fr-width paged ---
// Test fr box.
Hello #box(width: 1fr, rect(height: 0.7em, width: 100%)) World

--- box-fr-width-negative paged ---
// A negative fraction will produce negative spacing when the sum of fractions
// is positive, so this should look as follows:
// - only the red box for the first third of the width,
// - the red, green, and blue boxes in the middle third,
// - and only the green box in the final third
#set page(width: 40pt, margin: 0.5pt)
#set block(spacing: 5pt)

#block(height: 15pt, {
  box(width: 2fr, stroke: red, height: 100%)
  box(width: -1fr, stroke: blue, height: 50%)
  box(width: 2fr, stroke: green, height: 100%)
})

// A negative fraction will produce positive spacing when the sum of fractions
// is negative, so this should look identical.
#block(height: 15pt, {
  box(width: -2fr, stroke: red, height: 100%)
  box(width: 1fr, stroke: blue, height: 50%)
  box(width: -2fr, stroke: green, height: 100%)
})

--- block-fr-height paged ---
#set page(height: 100pt)
#rect(height: 10pt, width: 100%)
#align(center, block(height: 1fr, width: 20pt, stroke: 1pt))
#rect(height: 10pt, width: 100%)

--- block-fr-height-auto-width paged ---
// Test that the fr block can also expand its parent.
#set page(height: 100pt)
#set align(center)
#block(inset: 5pt, stroke: green)[
  #rect(height: 10pt)
  #block(height: 1fr, stroke: 1pt, inset: 5pt)[
    #set align(center + horizon)
    I am the widest
  ]
  #rect(height: 10pt)
]

--- block-fr-height-first-child paged ---
// Test that block spacing is not trimmed if only an fr block precedes it.
#set page(height: 100pt)
#rect(height: 1fr)
#rect()

--- block-fr-height-multiple paged ---
#set page(height: 100pt)
#rect(height: 1fr)
#rect()
#block(height: 1fr, line(length: 100%, angle: 90deg))

--- block-multiple-pages paged ---
// Test block over multiple pages.
#set page(height: 60pt)

First!

#block[
  But, soft! what light through yonder window breaks? It is the east, and Juliet
  is the sun.
]

--- block-multiple-pages-empty paged ---
#set page(height: 60pt)
A
#block(height: 30pt)
B

--- block-box-fill paged ---
#set page(height: 100pt)
#let words = lorem(18).split()
#block(inset: 8pt, width: 100%, fill: aqua, stroke: aqua.darken(30%))[
  #words.slice(0, 13).join(" ")
  #box(fill: teal, outset: 2pt)[tempor]
  #words.slice(13).join(" ")
]

--- block-spacing-basic paged ---
#set par(spacing: 10pt)
Hello

There

#block(spacing: 20pt)[Further down]

--- block-above-below-context paged empty ---
#context test(block.above, auto)
#set block(spacing: 20pt)
#context test(block.above, 20pt)
#context test(block.below, 20pt)

--- block-spacing-context paged ---
// The values for `above` and `below` might be different, so we cannot retrieve
// `spacing` directly
//
// Error: 16-23 function `block` does not contain field `spacing`
#context block.spacing

--- block-spacing-table paged ---
// Test that paragraph spacing loses against block spacing.
#set block(spacing: 100pt)
#show table: set block(above: 5pt, below: 5pt)
Hello
#table(columns: 4, fill: (x, y) => if calc.odd(x + y) { silver })[A][B][C][D]

--- block-spacing-maximum paged ---
// While we're at it, test the larger block spacing wins.
#set block(spacing: 0pt)
#show raw: set block(spacing: 15pt)
#show list: set block(spacing: 2.5pt)

```rust
fn main() {}
```

- List

Paragraph

--- block-spacing-collapse-text-style paged ---
// Test spacing collapsing with different font sizes.
#grid(columns: 2)[
  #text(size: 12pt, block(below: 1em)[A])
  #text(size: 8pt, block(above: 1em)[B])
][
  #text(size: 12pt, block(below: 1em)[A])
  #text(size: 8pt, block(above: 1.25em)[B])
]

--- block-fixed-height paged ---
#set page(height: 100pt)
#set align(center)

#lines(3)
#block(width: 80%, height: 60pt, fill: aqua)
#lines(2)
#block(
  breakable: false,
  width: 100%,
  inset: 4pt,
  fill: aqua,
  lines(3) + colbreak(),
)

--- block-consistent-width paged ---
// Test that block enforces consistent width across regions. Also use some
// introspection to check that measurement is working correctly.
#block(stroke: 1pt, inset: 5pt)[
  #align(right)[Hi]
  #colbreak()
  Hello @netwok
]

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- box-inset-ratio paged empty ---
#let body-width = 10pt
#context for inset in range(10).map(n => n / 10) {
  // If there's infinite available space, then:
  // ```
  // measured-width = body-width + measured-width × inset.
  // ```
  // (not counting truncation errors)
  let (width: measured-width) = measure(
    box(
      // Outset should not affect inset.
      outset: 137pt,
      inset: (left: 100% * inset),
      block(width: body-width)
    ),
    width: auto,
  )
  assert.eq(measured-width, body-width / (1 - inset))
}

--- block-sticky paged ---
#set page(height: 100pt)
#lines(3)
#block(sticky: true)[D]
#block(sticky: true)[E]
F

--- block-sticky-alone paged ---
#set page(height: 50pt)
#block(sticky: true)[A]

--- block-sticky-many paged ---
#set page(height: 80pt)
#set block(sticky: true)
#block[A]
#block[B]
#block[C]
#block[D]
E
#block[F]
#block[G]

--- block-sticky-colbreak paged ---
A
#block(sticky: true)[B]
#colbreak()
C

--- block-sticky-breakable paged ---
// Ensure that sticky blocks are still breakable.
#set page(height: 60pt)
#block(sticky: true, lines(4))
E

--- box-clip-rect paged ---
// Test box clipping with a rectangle
Hello #box(width: 1em, height: 1em, clip: false)[#rect(width: 3em, height: 3em, fill: red)]
world 1

Space

Hello #box(width: 1em, height: 1em, clip: true)[#rect(width: 3em, height: 3em, fill: red)]
world 2

--- block-clip-text paged ---
// Test clipping text
#block(width: 5em, height: 2em, clip: false, stroke: 1pt + black)[
  But, soft! what light through
]

#v(2em)

#block(width: 5em, height: 2em, clip: true, stroke: 1pt + black)[
  But, soft! what light through yonder window breaks? It is the east, and Juliet
  is the sun.
]

--- block-clip-svg-glyphs paged ---
// Test clipping svg glyphs
Emoji: #box(height: 0.5em, stroke: 1pt + black)[🐪, 🌋, 🏞]

Emoji: #box(height: 0.5em, clip: true, stroke: 1pt + black)[🐪, 🌋, 🏞]

--- block-clipping-multiple-pages paged ---
// Test block clipping over multiple pages.
#set page(height: 60pt)

First!

#block(height: 4em, clip: true, stroke: 1pt + black)[
  But, soft! what light through yonder window breaks? It is the east, and Juliet
  is the sun.
]

--- box-clip-radius paged ---
// Test clipping with `radius`.
#set page(height: 60pt)

#box(
  radius: 5pt,
  stroke: 2pt + black,
  width: 20pt,
  height: 20pt,
  clip: true,
  image("/assets/images/rhino.png", width: 30pt)
)

--- box-clip-radius-without-stroke paged ---
// Test clipping with `radius`, but without `stroke`.
#set page(height: 60pt)

#box(
  radius: 5pt,
  width: 20pt,
  height: 20pt,
  clip: true,
  image("/assets/images/rhino.png", width: 30pt)
)

--- box-clip-outset paged ---
// Test clipping with `outset`.
#set page(height: 60pt)

#box(
  outset: 5pt,
  stroke: 2pt + black,
  width: 20pt,
  height: 20pt,
  clip: true,
  image("/assets/images/rhino.png", width: 30pt)
)

--- box-baseline-context paged empty ---
#context test(box.baseline, (at: auto, shift: 0pt))

--- box-html-text html ---
Text #box[Span].

--- box-html-inline html ---
Text #box(html.strong[A])

--- box-html-multiple html ---
Text #box({
  html.strong[A]
  html.mark[B]
})

--- box-html-frame html ---
A
#box(html.frame(rect()))
#box(html.frame(rect()))
B

--- block-html-text html ---
Paragraph
#block[Div]

--- block-html-block html ---
Paragraph
#block(html.div[A])

--- block-html-inline html ---
Paragraph
#block(html.span[Top-level 1])
#block(html.span[Top-level 2])

--- block-html-multiple html ---
Paragraph
#block({
  html.strong[A]
  html.mark[B]
})

--- box-block-html html ---
Text #box(block(html.strong[A]))

--- block-box-html html ---
Text #block(box(html.div[A]))
Text #block(box(html.strong[B]))

--- block-block-html html ---
Text #block(block(html.div[A]))
Text #block(block(html.strong[B]))

--- block-html-frame html ---
Paragraph A
#html.frame(rect())
#html.frame(rect())
Paragraph B

--- block-display-html html ---
// Test how different HTML element's react to being promoted to block-level.
#block(html.script()) // display: none -> nothing
#block(html.div())    // display: block -> nothing
#block(html.table())  // display: table -> nothing
#block(html.li())     // display: list-item -> nothing
#block(html.span())   // display: inline -> block
#block(html.input())  // display: inline-block -> block
#block(html.slot())   // display: contents -> nothing
#block(html.ruby())   // display: ruby -> wrapped in div

--- block-invalid-html html ---
// These are currently wrapped in a div, but they are illegal in this context
// and should raise an error in the future.
#block(html.rt())
#block(html.thead())

--- box-invalid-html html ---
// This is illegal and should raise an error in the future. We do not even
// attempt to set a `display` property.
A #box(html.div()) B

--- container-layoutable-child paged ---
// Test box/block sizing with directly layoutable child.
//
// Ensure that the output respects the box size.
#let check(f) = f(
  width: 40pt, height: 25pt, fill: aqua,
  grid(rect(width: 5pt, height: 5pt, fill: blue)),
)

#stack(dir: ltr, spacing: 1fr, check(box), check(block))

--- issue-2128-block-width-box paged ---
// Test box in 100% width block.
#block(width: 100%, fill: red, box("a box"))
#block(width: 100%, fill: red, [#box("a box") #box()])

--- issue-2914-block-height-cut-off paged ---
// Ensure that breaking a block doesn't shrink its height.
#set page(height: 65pt)
#set block(fill: aqua, width: 25pt, height: 25pt, inset: 5pt)

#block[A]
#block[B]

--- issue-2914-block-fill-skip-nested paged ---
// Ensure that fill and stroke are skipped for an empty frame with a nested block.
#set page(height: 50pt)
A
#block(fill: aqua, stroke: blue, inset: 5pt, width: 100%, block[B])

--- issue-6304-block-skip-label paged ---
// Ensure that labeling is skipped for an empty orphan frame.
#set page(height: 60pt)
A
#block(sticky: true)[B]
#block[C] <label>

--- issue-6125-block-place-width-limited paged ---
// Ensure that the width of a placed block isn't limited by its siblings.
#set page(height: 70pt)
#let b = block({
  square(size: 20pt, fill: aqua)
  place(top, box(height: 10pt, width: 1fr, fill: blue))
})
#b
#b

--- issue-5296-block-sticky-in-block-at-top paged ---
#set page(height: 3cm)
#v(1.6cm)
#block(height: 2cm, breakable: true)[
  #block(sticky: true)[*A*]

  b
]

--- issue-5296-block-sticky-spaced-from-top-of-page paged ---
#set page(height: 3cm)
#v(2cm)

#block(sticky: true)[*A*]

b

--- issue-5296-block-sticky-weakly-spaced-from-top-of-page paged ---
#set page(height: 3cm)
#v(2cm, weak: true)

#block(sticky: true)[*A*]

b

--- issue-5262-block-negative-height paged ---
#block(height: -1pt)[]

--- issue-5262-block-negative-height-implicit paged ---
#set page(height: 10pt, margin: (top: 9pt))
#block(height: 100%)[]

--- issue-5262-block-negative-height-in-flow paged ---
// The contents after the block should be pushed upwards.
#set page(height: 60pt)
a
#block(height: -25pt)[b]
c

--- issue-6267-clip-anti-alias paged ---
#block(
  clip: true,
  radius: 100%,
  rect(fill: gray, height: 1cm, width: 1cm),
)
//@@ file tests/suite/layout_dir.typ
--- dir-from eval ---
#test(direction.from(left), ltr)
#test(direction.from(right), rtl)
#test(direction.from(top), ttb)
#test(direction.from(bottom), btt)

--- dir-from-invalid eval ---
// Error: 17-23 cannot convert this alignment to a side
#direction.from(center)

--- dir-to eval ---
#test(direction.to(left), rtl)
#test(direction.to(right), ltr)
#test(direction.to(top), btt)
#test(direction.to(bottom), ttb)

--- dir-to-invalid eval ---
// Error: 15-21 cannot convert this alignment to a side
#direction.to(center)

--- dir-axis eval ---
#test(ltr.axis(), "horizontal")
#test(rtl.axis(), "horizontal")
#test(ttb.axis(), "vertical")
#test(btt.axis(), "vertical")

--- dir-sign eval ---
#test(ltr.sign(), 1)
#test(rtl.sign(), -1)
#test(ttb.sign(), 1)
#test(btt.sign(), -1)

--- dir-start eval ---
#test(ltr.start(), left)
#test(rtl.start(), right)
#test(ttb.start(), top)
#test(btt.start(), bottom)

--- dir-end eval ---
#test(ltr.end(), right)
#test(rtl.end(), left)
#test(ttb.end(), bottom)
#test(btt.end(), top)

--- dir-inv eval ---
#test(ltr.inv(), rtl)
#test(rtl.inv(), ltr)
#test(ttb.inv(), btt)
#test(btt.inv(), ttb)
//@@ file tests/suite/layout_flow_flow.typ
--- flow-fr paged ---
#set page(height: 2cm)
#set text(white)
#rect(fill: forest)[
          #v(1fr)
  #h(1fr) Hi you!
]

--- issue-flow-overlarge-frames paged ---
// In this bug, the first line of the second paragraph was on its page alone an
// the rest moved down. The reason was that the second block resulted in
// overlarge frames because the region wasn't finished properly.
#set page(height: 70pt)
#block(lines(3))
#block(lines(5))

--- issue-flow-trailing-leading paged ---
// In this bug, the first part of the paragraph moved down to the second page
// because trailing leading wasn't trimmed, resulting in an overlarge frame.
#set page(height: 60pt)
#v(19pt)
#block[
  But, soft! what light through yonder window breaks?
  It is the east, and Juliet is the sun.
]

--- issue-flow-weak-spacing paged ---
// In this bug, there was a bit of space below the heading because weak spacing
// directly before a layout-induced column or page break wasn't trimmed.
#set page(height: 60pt)
#rect(inset: 0pt, columns(2)[
  Text
  #v(12pt)
  Hi
  #v(10pt, weak: true)
  At column break.
])

--- issue-flow-frame-placement paged ---
// In this bug, a frame intended for the second region ended up in the first.
#set page(height: 105pt)
#block(lorem(20))

--- issue-flow-layout-index-out-of-bounds paged ---
// This bug caused an index-out-of-bounds panic when layouting paragraphs needed
// multiple reorderings.
#set page(height: 200pt)
#lines(10)

#figure(placement: auto, block(height: 100%))

#lines(3)

#lines(3)

--- issue-3641-float-loop paged ---
// Flow layout should terminate!
#set page(height: 40pt)

= Heading
#lines(2)

--- issue-3355-metadata-weak-spacing paged ---
#set page(height: 50pt)
#block(width: 100%, height: 30pt, fill: aqua)
#metadata(none)
#v(10pt, weak: true)
Hi

--- issue-3866-block-migration paged ---
#set page(height: 120pt)
#set text(costs: (widow: 0%, orphan: 0%))
#v(50pt)
#columns(2)[
  #lines(6)
  #block(rect(width: 80%, height: 80pt), breakable: false)
  #lines(6)
]

--- issue-5024-spill-backlog paged ---
#set page(columns: 2, height: 50pt)
#columns(2)[Hello]
//@@ file tests/suite/layout_flow_footnote.typ
// Test footnotes.

--- footnote-basic paged html ---
#footnote[Hi]

--- footnote-space-collapsing paged html ---
// Test space collapsing before footnote.
A#footnote[A] \
A #footnote[A]

--- footnote-nested paged html ---
First \
Second #footnote[A, #footnote[B, #footnote[C]]]
Third #footnote[D, #footnote[E]] \
Fourth #footnote[F]

--- footnote-nested-break-across-pages paged ---
#set page(height: 80pt)
A #footnote([I: ] + lines(6) + footnote[II])
B #footnote[III]

--- footnote-entry paged ---
// Test customization.
#show footnote: set text(red)
#show footnote.entry: set text(8pt, style: "italic")
#set footnote.entry(
  indent: 0pt,
  gap: 0.6em,
  clearance: 0.3em,
  separator: repeat[.],
)

Beautiful footnotes. #footnote[Wonderful, aren't they?]

--- footnote-entry-html html ---
#show footnote.entry: it => {
  if it.note.body == [A] {
    [The A is replaced!]
  } else if it.note.body == [B] {
    none
  } else {
    show "C": emph
    it
  }
}

A #footnote[A]
B #footnote[B]
C #footnote[C]

--- footnote-container-set-rule-html html ---
// Set rule applies to everything.
//
// This is similar to page-marginal-style-text-set.
#set smartquote(quotes: ("[", "]"))
An "A" #footnote[A "B"]

--- footnote-container-show-set-rule-html html ---
// Set rule does not apply to footnote even though the par
// covers the whole document.
//
// This is similar to page-marginal-style-show-rule-with-set-page.
#show par: set smartquote(quotes: ("[", "]"))
An "A" #footnote[A "B"]

--- footnote-break-across-pages paged ---
#set page(height: 200pt)

#lines(2)
#footnote[ // 1
  I
  #footnote[II ...] // 2
]
#lines(6)
#footnote[III: #lines(8, "1")] // 3
#lines(6)
#footnote[IV: #lines(15, "1")] // 4
#lines(6)
#footnote[V] // 5

--- footnote-break-across-pages-block paged ---
#set page(height: 100pt)
#block[
  #lines(3) #footnote(lines(6, "1"))
  #footnote[Y]
  #footnote[Z]
]

--- footnote-break-across-pages-float paged ---
#set page(height: 180pt)

#lines(5)

#place(
  bottom,
  float: true,
  rect(height: 50pt, width: 100%, {
    footnote(lines(6, "1"))
    footnote(lines(2, "I"))
  })
)

#lines(5)

--- footnote-break-across-pages-nested paged ---
#set page(height: 120pt)
#block[
  #lines(4)
  #footnote[
    #lines(6, "1")
    #footnote(lines(3, "I"))
  ]
]

--- footnote-in-columns paged ---
#set page(height: 120pt, columns: 2)

#place(
  top + center,
  float: true,
  scope: "parent",
  clearance: 12pt,
  strong[Title],
)

#lines(3)
#footnote(lines(4, "1"))

#lines(2)
#footnote(lines(2, "1"))

--- footnote-in-list paged ---
#set page(height: 120pt)

- A #footnote[a]
- B #footnote[b]
- C #footnote[c]
- D #footnote[d]
- E #footnote[e]
- F #footnote[f]
- G #footnote[g]

--- footnote-block-at-end paged ---
#set page(height: 50pt)
A
#block(footnote[hello])

--- footnote-block-fr paged ---
#set page(height: 110pt)
A
#block(width: 100%, height: 1fr, fill: aqua)[
  B #footnote[I] #footnote[II]
]
C

--- footnote-float-priority paged ---
#set page(height: 100pt)

#lines(3)

#place(
  top,
  float: true,
  rect(height: 40pt)
)

#block[
  V
  #footnote[1]
  #footnote[2]
  #footnote[3]
  #footnote[4]
]

#lines(5)

--- footnote-in-caption paged ---
// Test footnote in caption.
Read the docs #footnote[https://typst.app/docs]!
#figure(
  image("/assets/images/graph.png", width: 70%),
  caption: [
    A graph #footnote[A _graph_ is a structure with nodes and edges.]
  ]
)
More #footnote[just for ...] footnotes #footnote[... testing. :)]

--- footnote-in-place paged ---
A
#place(top + right, footnote[A])
#figure(
  placement: bottom,
  caption: footnote[B],
  rect(),
)

--- footnote-duplicate paged ---
// Test duplicate footnotes.
#let lang = footnote[Languages.]
#let nums = footnote[Numbers.]

/ "Hello": A word #lang
/ "123": A number #nums

- "Hello" #lang
- "123" #nums

+ "Hello" #lang
+ "123" #nums

#table(
  columns: 2,
  [Hello], [A word #lang],
  [123], [A number #nums],
)

--- footnote-invariant paged ---
// Ensure that a footnote and the first line of its entry
// always end up on the same page.
#set page(height: 120pt)

#lines(5)

A #footnote(lines(6, "1"))

--- footnote-ref paged ---
// Test references to footnotes.
A footnote #footnote[Hi]<fn> \
A reference to it @fn

--- footnote-self-ref paged ---
// Error: 2-16 footnote cannot reference itself
#footnote(<fn>) <fn>

--- footnote-ref-multiple paged html ---
// Multiple footnotes are refs
First #footnote[A]<fn1> \
Second #footnote[B]<fn2> \
First ref @fn1 \
Third #footnote[C] \
Fourth #footnote[D]<fn4> \
Fourth ref @fn4 \
Second ref @fn2 \
Second ref again @fn2

--- footnote-ref-forward paged ---
// Forward reference
Usage @fn \
Definition #footnote[Hi]<fn>

--- footnote-ref-in-footnote paged ---
// Footnote ref in footnote
#footnote[Reference to next @fn]
#footnote[Reference to myself @fn]<fn>
#footnote[Reference to previous @fn]

--- footnote-styling paged ---
// Styling
#show footnote: text.with(fill: red)
Real #footnote[...]<fn> \
Ref @fn

--- footnote-ref-call paged ---
// Footnote call with label
#footnote(<fn>)
#footnote[Hi]<fn>
#ref(<fn>)
#footnote(<fn>)

--- footnote-in-table paged ---
// Test footnotes in tables. When the table spans multiple pages, the footnotes
// will all be after the table, but it shouldn't create any empty pages.
#set page(height: 100pt)

= Tables
#table(
  columns: 2,
  [Hello footnote #footnote[This is a footnote.]],
  [This is more text],
  [This cell
   #footnote[This footnote is not on the same page]
   breaks over multiple pages.],
  image("/assets/images/tiger.jpg"),
)

#table(
  columns: 3,
  ..range(1, 10)
    .map(numbering.with("a"))
    .map(v => upper(v) + footnote(v))
)

--- footnote-multiple-in-one-line paged ---
#set page(height: 100pt)
#v(50pt)
A #footnote[a]
B #footnote[b]

--- issue-1433-footnote-in-list paged ---
// Test that footnotes in lists do not produce extraneous page breaks. The list
// layout itself does not currently react to the footnotes layout, weakening the
// "footnote and its entry are on the same page" invariant somewhat, but at
// least there shouldn't be extra page breaks.
#set page(height: 100pt)
#block(height: 50pt, width: 100%, fill: aqua)

- #footnote[1]
- #footnote[2]

--- issue-footnotes-skip-first-page paged ---
// In this issue, we would get an empty page at the beginning because footnote
// layout didn't properly check for in_last.
#set page(height: 50pt)
#footnote[A]
#footnote[B]

--- issue-4454-footnote-ref-numbering paged ---
// Test that footnote references are numbered correctly.
A #footnote(numbering: "*")[B]<fn>, C @fn, D @fn, E @fn.

--- issue-5354-footnote-empty-frame-infinite-loop paged ---
// Test whether an empty footnote would cause infinite loop
#show footnote.entry: it => {}
#lorem(3) #footnote[A footnote]

--- issue-5256-multiple-footnotes-in-footnote paged ---
// Test whether all footnotes inside another footnote are listed.
#footnote[#footnote[A]#footnote[B]#footnote[C]]

--- issue-5435-footnote-migration-in-floats paged ---
// Test that a footnote should not prompt migration when in a float that was
// queued to the next page (due to the float being too large), even if the
// footnote does not fit, breaking the footnote invariant.
#set page(height: 50pt)

#place(
  top,
  float: true,
  {
    v(100pt)
    footnote[a]
  }
)
#place(
  top,
  float: true,
  footnote[b]
)

--- issue-5496-footnote-never-fits paged ---
// Test whether a footnote which is always too large would cause an infinite
// loop.
#set page(width: 20pt, height: 20pt)
#set footnote.entry(indent: 0pt)

#footnote(text(size: 15pt)[a] * 100)

--- issue-5496-footnote-in-float-never-fits paged ---
// Test whether an overlarge footnote in a float also does not cause an
// infinite loop.
#set page(width: 20pt, height: 20pt)

#place(
  top,
  float: true,
  footnote(text(size: 15pt)[a] * 100)
)

--- issue-5496-footnote-never-fits-multiple paged ---
// Test whether multiple overlarge footnotes are properly split up across
// pages.
#set page(width: 20pt, height: 20pt)
#set footnote.entry(indent: 0pt)

A

#footnote(text(size: 15pt)[a] * 100)
#footnote(text(size: 15pt)[b] * 100)
#footnote[Fit]

B

C

--- issue-5496-footnote-separator-never-fits paged ---
// Test whether an overlarge footnote separator does not cause an infinite
// loop and compiles.
#set page(height: 2em)
#set footnote.entry(separator: v(5em))

#footnote[]

--- footnote-custom-head-html html ---
#html.html({
  html.head()
  html.body[
    // Error: 12-32 footnotes are not currently supported in combination with a custom `<html>` or `<body>` element
    // Hint: 12-32 you can still use footnotes with a custom footnote show rule
    Hello #footnote[Footnote 1]
  ]
})

--- footnote-custom-head-html-show-none html ---
#show footnote: none
#html.html({
  html.head()
  html.body[
    Hello #footnote[Footnote 1]
  ]
})

--- footnote-bundle bundle ---
// These two documents have a shared footnote counter.
#document("a.html", footnote[A])
#document("b.pdf", footnote[B])

// From now on, each document has its own.
#show document: it => counter(footnote).update(()) + it
#document("standalone/c.html")[#footnote[C1], #footnote[C2]]
#document("standalone/d.html")[#footnote[D1], #footnote[D2]]
#document("standalone/e.pdf" )[#footnote[E1], #footnote[E2]]
//@@ file tests/suite/layout_flow_invisibles.typ
// Test out-of-flow items (place, counter updates, etc.) at the
// beginning of a block not creating a frame just for them.

--- flow-first-region-no-item paged ---
// No item in the first region.
#set page(height: 5cm, margin: 1cm)
No item in the first region.
#block(breakable: true, stroke: 1pt, inset: 0.5cm)[
  #rect(height: 2cm, fill: gray)
]

--- flow-first-region-counter-update paged ---
// Counter update in the first region.
#set page(height: 5cm, margin: 1cm)
Counter update.
#block(breakable: true, stroke: 1pt, inset: 0.5cm)[
  #counter("dummy").step()
  #rect(height: 2cm, fill: gray)
]

--- flow-first-region-placed paged ---
// Placed item in the first region.
#set page(height: 5cm, margin: 1cm)
Placed item in the first region.
#block(breakable: true, above: 1cm, stroke: 1pt, inset: 0.5cm)[
  #place(dx: -0.5cm, dy: -0.75cm, box(width: 200%)[OOF])
  #rect(height: 2cm, fill: gray)
]

--- flow-first-region-zero-sized-item paged ---
// In-flow item with size zero in the first region.
#set page(height: 5cm, margin: 1cm)
In-flow, zero-sized item.
#block(breakable: true, stroke: 1pt, inset: 0.4cm)[
  #set block(spacing: 0pt)
  #line(length: 0pt)
  #rect(height: 2cm, fill: gray)
  #line(length: 100%)
]

--- flow-first-region-counter-update-and-placed paged ---
// Counter update and placed item in the first region.
#set page(height: 5cm, margin: 1cm)
Counter update + place.
#block(breakable: true, above: 1cm, stroke: 1pt, inset: 0.5cm)[
  #counter("dummy").step()
  #place(dx: -0.5cm, dy: -0.75cm, box([OOF]))
  #rect(height: 2cm, fill: gray)
]

--- flow-first-region-counter-update-placed-and-line paged ---
// Mix-and-match all the previous ones.
#set page(height: 5cm, margin: 1cm)
Mix-and-match all the previous tests.
#block(breakable: true, above: 1cm, stroke: 1pt, inset: 0.5cm)[
  #counter("dummy").step()
  #place(dx: -0.5cm, dy: -0.75cm, box(width: 200%)[OOF])
  #line(length: 100%)
  #place(dy: 0.2em)[OOF]
  #rect(height: 2cm, fill: gray)
]
//@@ file tests/suite/layout_flow_orphan.typ
// Test that lines and headings doesn't become orphan.

--- flow-heading-no-orphan paged ---
#set page(height: 100pt)
#lines(4)

= Introduction
A

--- flow-par-no-orphan-and-widow-lines paged ---
#set page(width: 60pt, height: 140pt)
#set text(weight: 700)

// Fits fully onto the first page.
#set text(blue)
#lines(8)

// The first line would fit, but is moved to the second page.
#lines(6, "1")

// The second-to-last line is moved to the third page so that the last is isn't
// as lonely.
#set text(maroon)
#lines(4)

#lines(4, "1")

// All three lines go to the next page.
#set text(olive)
#lines(3)

--- flow-heading-no-orphan-with-footnote paged ---
// Ensure that a footnote in text after heading doesn't cause an orphan.
#set page(height: 80pt)
= A
#lines(1)

== B
#lines(1)#footnote[#lines(1)]

--- flow-heading-orphan-forced-with-footnote paged ---
// Ensure that a footnote in text after heading allows an orphan when there
// isn't enough space.
#set page(height: 60pt)
== A
#lines(1)#footnote[#lines(2)]

--- flow-widow-forced paged ---
// Ensure that a widow is allowed when the three lines don't all fit.
#set page(height: 50pt)
#lines(3)

--- issue-1445-widow-orphan-unnecessary-skip paged ---
// Ensure that widow/orphan prevention doesn't unnecessarily move things
// to another page.
#set page(width: 16cm)
#block(height: 30pt, fill: aqua, columns(2, lorem(19)))
//@@ file tests/suite/layout_flow_place.typ
// Test the `place` function.

--- place-basic paged ---
#set page("a8")
#place(bottom + center)[E]

= A
#place(right, rect(width: 1.8cm))
#lines(5)

#stack(
  rect(fill: eastern, height: 10pt, width: 100%),
  place(right, dy: 1.5pt)[ABC],
  rect(fill: conifer, height: 10pt, width: 80%),
  rect(fill: forest, height: 10pt, width: 100%),
  10pt,
  block[
    #place(center, dx: -7pt, dy: -5pt)[A]
    #place(center, dx: 7pt, dy: 5pt)[B]
    C #h(1fr) D
  ]
)

--- place-block-spacing paged ---
// Test how the placed element interacts with paragraph spacing around it.
#set page("a8", height: 60pt)

First

#place(bottom + right)[Placed]

Second

--- place-bottom-in-box paged ---
#box(
  fill: aqua,
  width: 30pt,
  height: 30pt,
  place(bottom,
    place(line(start: (0pt, 0pt), end: (20pt, 0pt), stroke: red + 3pt))
  )
)

--- place-horizon-in-boxes paged ---
#box(
  fill: aqua,
  width: 30pt,
  height: 30pt,
  {
    box(fill: yellow, {
      [Hello]
      place(horizon, line(start: (0pt, 0pt), end: (20pt, 0pt), stroke: red + 2pt))
    })
    place(horizon, line(start: (0pt, 0pt), end: (20pt, 0pt), stroke: green + 3pt))
  }
)

--- place-bottom-right-in-box paged ---
#box(fill: aqua)[
  #place(bottom + right)[Hi]
  Hello World \
  How are \
  you?
]

--- place-top-left-in-box paged ---
#box(fill: aqua)[
  #place(top + left, dx: 50%, dy: 50%)[Hi]
  #v(30pt)
  #line(length: 50pt)
]

--- place-float-flow-around paged ---
#set page(height: 80pt)
#set place(float: true)
#place(bottom + center, rect(height: 20pt))
#lines(4)

--- place-float-queued paged ---
#set page(height: 180pt)
#set figure(placement: auto)

#figure(rect(height: 60pt), caption: [I])
#figure(rect(height: 40pt), caption: [II])
#figure(rect(), caption: [III])
A
#figure(rect(), caption: [IV])

--- place-float-align-auto paged ---
#set page(height: 140pt)
#set place(auto, float: true, clearance: 5pt)

#place(rect[A])
#place(rect[B])
1 \ 2
#place(rect[C])
#place(rect[D])

--- place-float-delta paged ---
#place(top + center, float: true, dx: 10pt, rect[I])
A
#place(bottom + center, float: true, dx: -10pt, rect[II])

--- place-float-flow-size paged ---
#set page(width: auto, height: auto)
#set place(float: true, clearance: 5pt)

#place(bottom, rect(width: 80pt, height: 10pt))
#place(top + center, rect(height: 20pt))
#align(center)[A]
#pagebreak()
#align(center)[B]
#place(bottom, scope: "parent", rect(height: 10pt))

--- place-float-flow-size-alone paged ---
#set page(width: auto, height: auto)
#set place(float: true, clearance: 5pt)
#place(auto)[A]

--- place-float-fr paged ---
#set page(height: 120pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#place(top + center, rect[I])
#place(bottom + center, scope: "parent", rect[II])

A
#v(1fr)
B
#colbreak()
C
#align(bottom)[D]

--- place-float-rel-sizing paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#place(top + center, scope: "parent", rect[I])
#place(top + center, rect[II])

// This test result is not ideal: The first column takes 30% of the full page,
// while the second takes 30% of the remaining space since there is no concept
// of `full` for followup pages.
#set align(bottom)
#rect(width: 100%, height: 30%)
#rect(width: 100%, height: 30%)

--- place-float-block-backlog paged ---
#set page(height: 100pt)
#v(60pt)
#place(top, float: true, rect())
#list(.."ABCDEFGHIJ".clusters())

--- place-float-clearance-empty paged ---
// Check that we don't require space for clearance if there is no content.
#set page(height: 100pt)
#v(1fr)
#table(
  columns: (1fr, 1fr),
  lines(2),
  [],
  lines(8),
  place(auto, float: true, block(width: 100%, height: 100%, fill: aqua))
)

--- place-float-column-align-auto paged ---
#set page(height: 150pt, columns: 2)
#set place(auto, float: true, clearance: 10pt)
#set rect(width: 75%)

#place(rect[I])
#place(rect[II])
#place(rect[III])
#place(rect[IV])

#lines(6)

#place(rect[V])
#place(rect[VI])

--- place-float-column-queued paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 75%)
#set text(costs: (widow: 0%, orphan: 0%))

#lines(3)

#place(top, rect[I])
#place(top, rect[II])
#place(bottom, rect[III])

#lines(3)

--- place-float-twocolumn paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#place(top + center, scope: "parent", rect[I])
#place(top + center, rect[II])
#lines(4)
#place(top + center, rect[III])
#block(width: 100%, height: 70pt, fill: conifer)
#place(bottom + center, scope: "parent", rect[IV])
#place(bottom + center, rect[V])
#v(1pt, weak: true)
#block(width: 100%, height: 60pt, fill: aqua)

--- place-float-twocolumn-queued paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, scope: "parent", clearance: 10pt)
#let t(align, fill) = place(top + align, rect(fill: fill, height: 25pt))

#t(left, aqua)
#t(center, forest)
#t(right, conifer)
#lines(7)

--- place-float-twocolumn-align-auto paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#place(auto, scope: "parent", rect[I]) // Should end up `top`
#lines(4)
#place(auto, scope: "parent", rect[II])  // Should end up `bottom`
#lines(4)

--- place-float-twocolumn-fits paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#lines(6)
#place(auto, scope: "parent", rect[I])
#lines(12, "1")

--- place-float-twocolumn-fits-not paged ---
#set page(height: 100pt, columns: 2)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#lines(10)
#place(auto, scope: "parent", rect[I])
#lines(10, "1")

--- place-float-threecolumn paged ---
#set page(height: 100pt, columns: 3)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

#place(bottom + center, scope: "parent", rect[I])
#lines(21)
#place(top + center, scope: "parent", rect[II])

--- place-float-threecolumn-block-backlog paged ---
#set page(height: 100pt, columns: 3)
#set place(float: true, clearance: 10pt)
#set rect(width: 70%)

// The most important part of this test is that we get the backlog of the
// conifer (green) block right.
#place(top + center, scope: "parent", rect[I])
#block(fill: aqua, width: 100%, height: 70pt)
#block(fill: conifer, width: 100%, height: 160pt)
#place(bottom + center, scope: "parent", rect[II])
#place(top, rect(height: 40%)[III])
#block(fill: yellow, width: 100%, height: 60pt)

--- place-float-counter paged ---
#let c = counter("c")
#let cd = context c.display()

#set page(
  height: 100pt,
  margin: (y: 20pt),
  header: [H: #cd],
  footer: [F: #cd],
  columns: 2,
)

#let t(align, scope: "column", n) = place(
  align,
  float: true,
  scope: scope,
  clearance: 10pt,
  line(length: 100%) + c.update(n),
)

#t(bottom, 6)
#cd
#t(top, 3)
#colbreak()
#cd
#t(scope: "parent", bottom, 11)
#colbreak()
#cd
#t(top, 12)

--- place-float-missing paged ---
// Error: 2-20 automatic positioning is only available for floating placement
// Hint: 2-20 you can enable floating placement with `place(float: true, ..)`
#place(auto)[Hello]

--- place-float-center-horizon paged ---
// Error: 2-45 vertical floating placement must be `auto`, `top`, or `bottom`
#place(center + horizon, float: true)[Hello]

--- place-float-horizon paged ---
// Error: 2-36 vertical floating placement must be `auto`, `top`, or `bottom`
#place(horizon, float: true)[Hello]

--- place-float-default paged ---
// Error: 2-27 vertical floating placement must be `auto`, `top`, or `bottom`
#place(float: true)[Hello]

--- place-float-right paged ---
// Error: 2-34 vertical floating placement must be `auto`, `top`, or `bottom`
#place(right, float: true)[Hello]

--- place-flush paged ---
#set page(height: 120pt)
#let floater(align, height) = place(
  align,
  float: true,
  rect(width: 100%, height: height),
)

#floater(top, 30pt)
A

#floater(bottom, 50pt)
#place.flush()
B // Should be on the second page.

--- place-flush-figure paged ---
#set page(height: 120pt)
#let floater(align, height, caption) = figure(
  placement: align,
  caption: caption,
  rect(width: 100%, height: height),
)

#floater(top, 30pt)[I]
A

#floater(bottom, 50pt)[II]
#place.flush()
B // Should be on the second page.

--- issue-place-base paged ---
// Test that placement is relative to container and not itself.
#set page(height: 80pt, margin: 0pt)
#place(right, dx: -70%, dy: 20%, [First])
#place(left, dx: 20%, dy: 60%, [Second])
#place(center + horizon, dx: 25%, dy: 25%, [Third])

--- issue-1368-place-pagebreak paged ---
// Test placing on an already full page.
// It shouldn't result in a page break.
#set page(height: 40pt)
#block(height: 100%)
#place(bottom + right)[Hello world]

--- issue-2199-place-spacing-bottom paged ---
// Test that placed elements don't add extra block spacing.
#show figure: set block(spacing: 4em)

Paragraph before float.
#figure(rect(), placement: bottom)
Paragraph after float.

--- issue-2199-place-spacing-default paged ---
#show place: set block(spacing: 4em)

Paragraph before place.
#place(rect())
Paragraph after place.

--- issue-2595-float-overlap paged ---
#set page(height: 80pt)

1
#place(auto, float: true, block(height: 100%, width: 100%, fill: aqua))
#place(auto, float: true, block(height: 100%, width: 100%, fill: red))
#lines(7)
//@@ file tests/suite/layout_grid_cell.typ
// Test basic styling using the grid.cell element.

--- grid-cell-override paged ---
// Cell override
#grid(
  align: left,
  fill: red,
  stroke: blue,
  inset: 5pt,
  columns: 2,
  [AAAAA], [BBBBB],
  [A], [B],
  grid.cell(align: right)[C], [D],
  align(right)[E], [F],
  align(horizon)[G], [A\ A\ A],
  grid.cell(align: horizon)[G2], [A\ A\ A],
  grid.cell(inset: 0pt)[I], [F],
  [H], grid.cell(fill: blue)[J]
)

--- grid-cell-show paged ---
// Cell show rule
#show grid.cell: it => [Zz]

#grid(
  align: left,
  fill: red,
  stroke: blue,
  inset: 5pt,
  columns: 2,
  [AAAAA], [BBBBB],
  [A], [B],
  grid.cell(align: right)[C], [D],
  align(right)[E], [F],
  align(horizon)[G], [A\ A\ A]
)

--- grid-cell-show-and-override paged ---
#show grid.cell: it => (it.align, it.fill)
#grid(
  align: left,
  row-gutter: 5pt,
  [A],
  grid.cell(align: right)[B],
  grid.cell(fill: aqua)[B],
)

--- grid-cell-set paged ---
// Cell set rules
#set grid.cell(align: center)
#show grid.cell: it => (it.align, it.fill, it.inset)
#set grid.cell(inset: 20pt)
#grid(
  align: left,
  row-gutter: 5pt,
  [A],
  grid.cell(align: right)[B],
  grid.cell(fill: aqua)[B],
)

--- grid-cell-folding paged ---
// Test folding per-cell properties (align and inset)
#grid(
  columns: (1fr, 1fr),
  rows: (2.5em, auto),
  align: right,
  inset: 5pt,
  fill: (x, y) => (green, aqua).at(calc.rem(x + y, 2)),
  [Top], grid.cell(align: bottom)[Bot],
  grid.cell(inset: (bottom: 0pt))[Bot], grid.cell(inset: (bottom: 0pt))[Bot]
)

--- grid-cell-align-override paged ---
// Test overriding outside alignment
#set align(bottom + right)
#grid(
  columns: (1fr, 1fr),
  rows: 2em,
  align: auto,
  fill: green,
  [BR], [BR],
  grid.cell(align: left, fill: aqua)[BL], grid.cell(align: top, fill: red.lighten(50%))[TR]
)

--- grid-cell-various-overrides paged ---
#grid(
  columns: 2,
  fill: red,
  align: left,
  inset: 5pt,
  [ABC], [ABC],
  grid.cell(fill: blue)[C], [D],
  grid.cell(align: center)[E], [F],
  [G], grid.cell(inset: 0pt)[H]
)

--- grid-cell-show-emph paged ---
#{
  show grid.cell: emph
  grid(
    columns: 2,
    gutter: 3pt,
    [Hello], [World],
    [Sweet], [Italics]
  )
}

--- grid-cell-show-based-on-position paged ---
// Style based on position
#{
  show grid.cell: it => {
    if it.y == 0 {
      strong(it)
    } else if it.x == 1 {
      emph(it)
    } else {
      it
    }
  }
  grid(
    columns: 3,
    gutter: 3pt,
    [Name], [Age], [Info],
    [John], [52], [Nice],
    [Mary], [50], [Cool],
    [Jake], [49], [Epic]
  )
}

--- table-cell-in-grid eval ---
// Error: 7-19 cannot use `table.cell` as a grid cell
// Hint: 7-19 use `grid.cell` instead
#grid(table.cell[])

--- issue-5723-grid-heading-numbering paged ---
#set heading(numbering: "1.1.")
#set page(width: 150pt, height: 3.5cm)

#table(
  columns: (1fr, 2fr),
  [= A],
  [= B],
  [
    = C
    #lines(4)
    = D
  ],
  table(
    columns: (1fr, 1fr),
    ..([
      = X
      #lines(2)
      = Y
      #lines(2)
    ],) * 2
  ),
  [= E],
  [= F]
)

--- issue-7188-grid-counter-order paged ---
#set page(height: 1cm)

#let word-numbering(body) = {
  let num = counter("_linenumbered")
  let word-label = <_word>
  show word-label: _ => {
    num.step()
    box(width: 0pt, super(numbering("1", num.get().first())))
  }
  show regex("\\w+\\.?"): it => it + [#metadata(none)#word-label]
  body
}

#grid(
  columns: 1,
  word-numbering(lorem(8))
)
//@@ file tests/suite/layout_grid_colspan.typ
--- grid-colspan paged ---
#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  grid.cell(colspan: 4)[*Full Header*],
  grid.cell(colspan: 2, fill: orange)[*Half*],
  grid.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], grid.cell(colspan: 3, fill: orange.darken(10%))[6],
  grid.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], grid.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  table.cell(colspan: 4)[*Full Header*],
  table.cell(colspan: 2, fill: orange)[*Half*],
  table.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], table.cell(colspan: 3, fill: orange.darken(10%))[6],
  table.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], table.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

--- grid-colspan-gutter paged ---
#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  grid.cell(colspan: 4)[*Full Header*],
  grid.cell(colspan: 2, fill: orange)[*Half*],
  grid.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], grid.cell(colspan: 3, fill: orange.darken(10%))[6],
  grid.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], grid.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  table.cell(colspan: 4)[*Full Header*],
  table.cell(colspan: 2, fill: orange)[*Half*],
  table.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], table.cell(colspan: 3, fill: orange.darken(10%))[6],
  table.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], table.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

--- grid-colspan-thick-stroke paged ---
#set page(width: 300pt)
#table(
  columns: (2em, 2em, auto, auto),
  stroke: 5pt,
  [A], [B], [C], [D],
  table.cell(colspan: 4, lorem(20)),
  [A], table.cell(colspan: 2)[BCBCBCBC], [D]
)

--- grid-colspan-out-of-bounds paged ---
#grid(
  columns: 3,
  // Error: 8-32 cell's colspan would cause it to exceed the available column(s)
  // Hint: 8-32 try placing the cell in another position or reducing its colspan
  [a], grid.cell(colspan: 3)[b]
)

--- grid-colspan-overlap paged ---
#grid(
  columns: 3,
  grid.cell(x: 2, y: 0)[x],
  // Error: 8-32 cell would span a previously placed cell at column 2, row 0
  // Hint: 8-32 try specifying your cells in a different order or reducing the cell's rowspan or colspan
  [a], grid.cell(colspan: 2)[b]
)

--- grid-colspan-over-all-fr-columns paged ---
// Colspan over all fractional columns shouldn't expand auto columns on finite pages
#table(
  columns: (1fr, 1fr, auto),
  [A], [B], [C],
  [D], [E], [F]
)
#table(
  columns: (1fr, 1fr, auto),
  table.cell(colspan: 3, lorem(8)),
  [A], [B], [C],
  [D], [E], [F]
)

--- grid-colspan-over-some-fr-columns paged ---
// Colspan over only some fractional columns will not trigger the heuristic, and
// the auto column will expand more than it should. The table looks off, as a result.
#table(
  columns: (1fr, 1fr, auto),
  [], table.cell(colspan: 2, lorem(8)),
  [A], [B], [C],
  [D], [E], [F]
)

--- grid-colspan-over-all-fr-columns-page-width-auto paged ---
// On infinite pages, colspan over all fractional columns SHOULD expand auto columns
#set page(width: auto)
#table(
  columns: (1fr, 1fr, auto),
  [A], [B], [C],
  [D], [E], [F]
)
#table(
  columns: (1fr, 1fr, auto),
  table.cell(colspan: 3, lorem(8)),
  [A], [B], [C],
  [D], [E], [F]
)

--- grid-colspan-multiple-regions paged ---
// Test multiple regions
#set page(height: 5em)
#grid(
  stroke: red,
  fill: aqua,
  columns: 4,
  [a], [b], [c], [d],
  [a], grid.cell(colspan: 2)[e, f, g, h, i], [f],
  [e], [g], grid.cell(colspan: 2)[eee\ e\ e\ e],
  grid.cell(colspan: 4)[eeee e e e]
)

--- issue-6399-grid-cell-colspan-set-rule paged ---
#set grid.cell(colspan: 2)
#grid(columns: 3, [hehe])
//@@ file tests/suite/layout_grid_footers.typ
--- grid-footer paged ---
#set page(width: auto, height: 15em)
#set text(6pt)
#set table(inset: 2pt, stroke: 0.5pt)
#table(
  columns: 5,
  align: center + horizon,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow)
  ),
  ..range(0, 5).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten(),
  table.footer(
    table.hline(start: 2, end: 3, stroke: yellow),
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.cell(colspan: 5)[*Cool Zone*]
  )
)

--- grid-footer-gutter-and-no-repeat paged ---
// Gutter & no repetition
#set page(width: auto, height: 16em)
#set text(6pt)
#set table(inset: 2pt, stroke: 0.5pt)
#table(
  columns: 5,
  gutter: 2pt,
  align: center + horizon,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow)
  ),
  ..range(0, 5).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten(),
  table.footer(
    repeat: false,
    table.hline(start: 2, end: 3, stroke: yellow),
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.cell(colspan: 5)[*Cool Zone*]
  )
)

--- grid-cell-override-in-header-and-footer paged ---
#table(
  table.header(table.cell(stroke: red)[Hello]),
  table.footer(table.cell(stroke: aqua)[Bye]),
)

--- grid-cell-override-in-header-and-footer-with-gutter paged ---
#table(
  gutter: 3pt,
  table.header(table.cell(stroke: red)[Hello]),
  table.footer(table.cell(stroke: aqua)[Bye]),
)

--- grid-footer-top-stroke paged ---
// Footer's top stroke should win when repeated, but lose at the last page.
#set page(height: 10em)
#table(
  stroke: green,
  table.header(table.cell(stroke: red)[Hello]),
  table.cell(stroke: yellow)[Hi],
  table.cell(stroke: yellow)[Bye],
  table.cell(stroke: yellow)[Ok],
  table.footer[Bye],
)

--- grid-footer-relative-row-sizes paged ---
// Relative lengths
#set page(height: 10em)
#table(
  rows: (30%, 30%, auto),
  [C],
  [C],
  table.footer[*A*][*B*],
)

--- grid-footer-cell-with-y paged ---
#grid(
  grid.footer(grid.cell(y: 2)[b]),
  grid.cell(y: 0)[a],
  grid.cell(y: 1)[c],
)

--- grid-footer-cell-with-x paged ---
#grid(
  columns: 2,
  stroke: black,
  inset: 5pt,
  grid.cell(x: 1)[a],
  // Error: 3-56 footer must end at the last row
  grid.footer(grid.cell(x: 0)[b1], grid.cell(x: 0)[b2]),
  // This should skip the footer
  grid.cell(x: 1)[c]
)

--- grid-footer-no-expand-with-col-and-row-pos-cell paged ---
#grid(
  columns: 2,
  [a], [],
  [b], [],
  fill: (_, y) => if calc.odd(y) { blue } else { red },
  inset: 5pt,
  grid.cell(x: 1, y: 3, rowspan: 4)[b],
  grid.cell(y: 2, rowspan: 2)[a],
  grid.footer(),
  // Error: 3-27 cell would conflict with footer also spanning row 7
  // Hint: 3-27 try reducing the cell's rowspan or moving the footer
  grid.cell(x: 1, y: 7)[d],
)

--- grid-footer-no-expand-with-row-pos-cell paged ---
#grid(
  columns: 2,
  [a], [],
  [b], [],
  fill: (_, y) => if calc.odd(y) { blue } else { red },
  inset: 5pt,
  grid.cell(x: 1, y: 3, rowspan: 4)[b],
  grid.cell(y: 2, rowspan: 2)[a],
  grid.footer(),
  // Error: 3-33 cell would conflict with footer also spanning row 7
  // Hint: 3-33 try reducing the cell's rowspan or moving the footer
  grid.cell(y: 6, rowspan: 2)[d],
)

--- grid-footer-moved-to-bottom-of-rowspans paged ---
#grid(
  columns: 2,
  [a], [],
  [b], [],
  stroke: red,
  inset: 5pt,
  grid.cell(x: 1, y: 3, rowspan: 4)[b],
  grid.cell(y: 2, rowspan: 2)[a],
  grid.footer(),
  grid.cell(y: 4)[d],
  grid.cell(y: 5)[e],
  grid.cell(y: 6)[f],
)

--- grid-footer-not-at-last-row paged ---
#grid(
  // Error: 3-19 footer must end at the last row
  grid.footer([a]),
  [b],
)

--- grid-footer-not-at-last-row-two-columns paged ---
#grid(
  columns: 2,
  // Error: 3-19 footer must end at the last row
  grid.footer([a]),
  [b],
)

--- grid-footer-overlap paged ---
#grid(
  columns: 2,
  grid.header(),
  grid.footer(grid.cell(y: 2)[a]),
  // Error: 3-39 cell would conflict with footer also spanning row 2
  // Hint: 3-39 try reducing the cell's rowspan or moving the footer
  grid.cell(x: 1, y: 1, rowspan: 2)[a],
)

--- grid-footer-rowbreak-line paged ---
#grid(
  columns: 1,
  [a],
  grid.hline(stroke: red),
  grid.footer([b]),
  grid.hline(stroke: 3pt),
)

--- grid-footer-multiple paged ---
#grid(
  [a],
  grid.footer([a]),
  // Error: 3-19 cannot have more than one footer
  grid.footer([b]),
)

--- table-footer-in-grid eval ---
#grid(
  [a],
  // Error: 3-20 cannot use `table.footer` as a grid footer
  // Hint: 3-20 use `grid.footer` instead
  table.footer([a]),
)

--- grid-footer-in-table eval ---
#table(
  [a],
  // Error: 3-19 cannot use `grid.footer` as a table footer
  // Hint: 3-19 use `table.footer` instead
  grid.footer([a]),
)

--- grid-footer-in-grid-header eval ---
// Error: 14-28 cannot place a grid footer within another footer or header
#grid.header(grid.footer[a])

--- table-footer-in-grid-header eval ---
// Error: 14-29 cannot place a table footer within another footer or header
#grid.header(table.footer[a])

--- grid-footer-in-table-header eval ---
// Error: 15-29 cannot place a grid footer within another footer or header
#table.header(grid.footer[a])

--- table-footer-in-table-header eval ---
// Error: 15-30 cannot place a table footer within another footer or header
#table.header(table.footer[a])

--- grid-footer-in-grid-footer eval ---
// Error: 14-28 cannot place a grid footer within another footer or header
#grid.footer(grid.footer[a])

--- table-footer-in-grid-footer eval ---
// Error: 14-29 cannot place a table footer within another footer or header
#grid.footer(table.footer[a])

--- grid-footer-in-table-footer eval ---
// Error: 15-29 cannot place a grid footer within another footer or header
#table.footer(grid.footer[a])

--- table-footer-in-table-footer eval ---
// Error: 15-30 cannot place a table footer within another footer or header
#table.footer(table.footer[a])

--- grid-header-in-grid-footer eval ---
// Error: 14-28 cannot place a grid header within another header or footer
#grid.footer(grid.header[a])

--- table-header-in-grid-footer eval ---
// Error: 14-29 cannot place a table header within another header or footer
#grid.footer(table.header[a])

--- grid-header-in-table-footer eval ---
// Error: 15-29 cannot place a grid header within another header or footer
#table.footer(grid.header[a])

--- table-header-in-table-footer eval ---
// Error: 15-30 cannot place a table header within another header or footer
#table.footer(table.header[a])

--- grid-header-footer-block-with-fixed-height paged ---
#set page(height: 17em)
#table(
  rows: (auto, 2.5em, auto),
  table.header[*Hello*][*World*],
  block(width: 2em, height: 10em, fill: red),
  table.footer[*Bye*][*World*],
)

--- grid-header-footer-and-rowspan-non-contiguous-1 paged ---
// Rowspan sizing algorithm doesn't do the best job at non-contiguous content
// ATM.
#set page(height: 20em)

#table(
  rows: (auto, 2.5em, 2em, auto, 5em, 2em, 2.5em),
  table.header[*Hello*][*World*],
  table.cell(rowspan: 3, lorem(20)),
  table.footer[*Ok*][*Bye*],
)

--- grid-header-footer-and-rowspan-non-contiguous-2 paged ---
// This should look right
#set page(height: 20em)

#table(
  rows: (auto, 2.5em, 2em, auto),
  gutter: 3pt,
  table.header[*Hello*][*World*],
  table.cell(rowspan: 3, lorem(20)),
  table.footer[*Ok*][*Bye*],
)
--- grid-header-and-footer-lack-of-space paged ---
// Test lack of space for header + text.
#set page(height: 9em + 2.5em + 1.5em)

#table(
  rows: (auto, 2.5em, auto, auto, 10em, 2.5em, auto),
  gutter: 3pt,
  table.header[*Hello*][*World*],
  table.cell(rowspan: 3, lorem(30)),
  table.footer[*Ok*][*Bye*],
)

--- grid-header-and-footer-orphan-prevention paged ---
// Orphan header prevention test
#set page(height: 13em)
#v(8em)
#grid(
  columns: 3,
  gutter: 5pt,
  grid.header(
    [*Mui*], [*A*], grid.cell(rowspan: 2, fill: orange)[*B*],
    [*Header*], [*Header* #v(0.1em)],
  ),
  ..([Test], [Test], [Test]) * 7,
  grid.footer(
    [*Mui*], [*A*], grid.cell(rowspan: 2, fill: orange)[*B*],
    [*Footer*], [*Footer* #v(0.1em)],
  ),
)

--- grid-header-and-footer-empty paged ---
// Empty footer should just be a repeated blank row
#set page(height: 8em)
#table(
  columns: 4,
  align: center + horizon,
  table.header(),
  ..range(0, 2).map(i => (
    [John \##i],
    table.cell(stroke: green)[123],
    table.cell(stroke: blue)[456],
    [789]
  )).flatten(),
  table.footer(),
)

--- grid-header-and-footer-containing-rowspan paged ---
// When a footer has a rowspan with an empty row, it should be displayed
// properly
#set page(height: 14em, width: auto)

#let count = counter("g")
#table(
  rows: (auto, 2em, auto, auto),
  table.header(
    [eeec],
    table.cell(rowspan: 2, count.step() + context count.display()),
  ),
  [d],
  block(width: 5em, fill: yellow, lorem(7)),
  [d],
  table.footer(
    [eeec],
    table.cell(rowspan: 2, count.step() + context count.display()),
  )
)
#context count.display()

--- grid-nested-with-footers paged ---
// Nested table with footer should repeat both footers
#set page(height: 10em, width: auto)
#table(
  table(
    [a\ b\ c\ d],
    table.footer[b],
  ),
  table.footer[a],
)

--- grid-nested-footers paged ---
#set page(height: 12em, width: auto)
#table(
  [a\ b\ c\ d],
  table.footer(table(
    [c],
    [d],
    table.footer[b],
  ))
)

--- grid-footer-rowspan paged ---
// General footer-only tests
#set page(height: 9em)
#table(
  columns: 2,
  [a], [],
  [b], [],
  [c], [],
  [d], [],
  [e], [],
  table.footer(
    [*Ok*], table.cell(rowspan: 2)[test],
    [*Thanks*]
  )
)

--- grid-footer-bare-1 paged ---
#set page(height: 5em)
#table(
  table.footer[a][b][c]
)

--- grid-footer-bare-2 paged ---
#table(table.footer[a][b][c])

#table(
  gutter: 3pt,
  table.footer[a][b][c]
)

--- grid-footer-repeatable-unbreakable paged ---
#set page(height: 8em, width: auto)
#table(
  [h],
  table.footer(
    [a],
    [b],
    [c],
  )
)

--- grid-footer-non-repeatable-unbreakable paged ---
#set page(height: 8em, width: auto)
#table(
  [h],
  table.footer(
    [a],
    [b],
    [c],
    repeat: false,
  )
)

--- grid-footer-stroke-edge-cases paged ---
// Test footer stroke priority edge case
#set page(height: 10em)
#table(
  columns: 2,
  stroke: black,
  ..(table.cell(stroke: aqua)[d],) * 8,
  table.footer(
    table.cell(rowspan: 2, colspan: 2)[a],
    [c], [d]
  )
)

--- grid-footer-hline-and-vline-1 paged ---
// Footer should appear at the bottom. Red line should be above the footer.
// Green line should be on the left border.
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 2,
  inset: 1.5pt,
  table.cell(y: 0)[a],
  table.cell(x: 1, y: 1)[a],
  table.cell(y: 2)[a],
  table.footer(
    table.hline(stroke: red),
    table.vline(stroke: green),
    [b],
    [c]
  ),
)

--- grid-footer-hline-and-vline-2 paged ---
// Table should be just one row. [c] appears at the third column.
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 3,
  inset: 1.5pt,
  table.footer(
    table.cell(y: 0)[a],
    table.hline(stroke: red),
    table.hline(y: 1, stroke: aqua),
    table.cell(y: 0)[b],
    [c]
  )
)

--- grid-footer-top-hlines-with-only-row-pos-cell paged ---
// Top hlines should attach to the top of the footer.
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 3,
  inset: 2.5pt,
  table.footer(
    table.hline(stroke: red),
    table.vline(stroke: blue),
    table.cell(x: 2, y: 2)[a],
    table.hline(stroke: 3pt),
    table.vline(stroke: 3pt),
  )
)

--- grid-footer-top-hlines-with-row-and-auto-pos-cell paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 3,
  inset: 2.5pt,
  table.footer(
    table.hline(stroke: red),
    table.vline(stroke: blue),
    table.cell(x: 2, y: 2)[a],
    [b],
    table.hline(stroke: 3pt),
    table.vline(stroke: 3pt),
  )
)

--- grid-footer-below-rowspans paged ---
// Footer should go below the rowspans.
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 2,
  inset: 1.5pt,
  table.cell(rowspan: 2)[a], table.cell(rowspan: 2)[b],
  table.footer()
)

--- grid-footer-row-pos-cell-inside-conflicts-with-row-before paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 3,
  inset: 1.5pt,
  table.cell(y: 0)[a],
  table.footer(
    table.hline(stroke: red),
    table.hline(y: 1, stroke: aqua),
    // Error: 5-24 cell would cause footer to expand to non-empty row 0
    // Hint: 5-24 try moving its cells to available rows
    table.cell(y: 0)[b],
    [c]
  )
)

--- grid-footer-auto-pos-cell-inside-conflicts-with-row-after paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 2,
  inset: 1.5pt,
  table.cell(y: 1)[a],
  table.footer(
    [b], [c],
    // Error: 6-7 cell would cause footer to expand to non-empty row 1
    // Hint: 6-7 try moving its cells to available rows
    [d],
  ),
)

--- grid-footer-row-pos-cell-inside-conflicts-with-row-after paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 2,
  inset: 1.5pt,
  table.cell(y: 2)[a],
  table.footer(
    [b], [c],
    // Error: 5-24 cell would cause footer to expand to non-empty row 2
    // Hint: 5-24 try moving its cells to available rows
    table.cell(y: 3)[d],
  ),
)

--- grid-footer-conflicts-with-empty-header paged ---
#table(
  columns: 2,
  table.header(),
  table.footer(
    // Error: 5-24 cell would cause footer to expand to non-empty row 0
    // Hint: 5-24 try moving its cells to available rows
    table.cell(y: 0)[a]
  ),
)

--- issue-5359-column-override-stays-inside-footer paged ---
#table(
  columns: 3,
  [Outside],
  table.footer(
    [A], table.cell(x: 1)[B], [C],
    table.cell(x: 1)[D],
  ),
)

--- issue-6666-auto-hlines-around-footer paged ---
#table(
	columns: 2,
	table.hline(stroke: 2pt + blue),
	table.footer([*foo*], [*bar*]),
	table.hline(stroke: 8pt),
)
//@@ file tests/suite/layout_grid_grid.typ
// Test grid layouts.

--- grid-columns-sizings-rect paged ---
#let cell(width, color) = rect(width: width, height: 2cm, fill: color)
#set page(width: 100pt, height: 140pt)
#grid(
  columns: (auto, 1fr, 3fr, 0.25cm, 3%, 2mm + 10%),
  cell(0.5cm, rgb("2a631a")),
  cell(100%,  forest),
  cell(100%,  conifer),
  cell(100%,  rgb("ff0000")),
  cell(100%,  rgb("00ff00")),
  cell(80%,   rgb("00faf0")),
  cell(1cm,   rgb("00ff00")),
  cell(0.5cm, rgb("2a631a")),
  cell(100%,  forest),
  cell(100%,  conifer),
  cell(100%,  rgb("ff0000")),
  cell(100%,  rgb("00ff00")),
)

--- grid-gutter-fr paged ---
#set rect(inset: 0pt)
#grid(
  columns: (auto, auto, 40%),
  column-gutter: 1fr,
  row-gutter: 1fr,
  rect(fill: eastern)[dddaa aaa aaa],
  rect(fill: conifer)[ccc],
  rect(fill: rgb("dddddd"))[aaa],
)

--- grid-row-sizing-manual-align paged ---
#set page(height: 3cm, margin: 0pt)
#grid(
  columns: (1fr,),
  rows: (1fr, auto, 2fr),
  [],
  align(center)[A bit more to the top],
  [],
)

--- grid-finance paged ---
// Test using the `grid` function to create a finance table.
#set page(width: 11cm, height: 2.5cm)
#grid(
  columns: 5,
  column-gutter: (2fr, 1fr, 1fr),
  row-gutter: 6pt,
  [*Quarter*],
  [Expenditure],
  [External Revenue],
  [Financial ROI],
  [_total_],
  [*Q1*],
  [173,472.57 \$],
  [472,860.91 \$],
  [51,286.84 \$],
  [_350,675.18 \$_],
  [*Q2*],
  [93,382.12 \$],
  [439,382.85 \$],
  [-1,134.30 \$],
  [_344,866.43 \$_],
  [*Q3*],
  [96,421.49 \$],
  [238,583.54 \$],
  [3,497.12 \$],
  [_145,659.17 \$_],
)
// Test grid cells that overflow to the next region.

--- grid-cell-breaking paged ---
#set page(width: 5cm, height: 3cm)
#grid(
  columns: 2,
  row-gutter: 8pt,
  [Lorem ipsum dolor sit amet.

  Aenean commodo ligula eget dolor. Aenean massa. Penatibus et magnis.],
  [Text that is rather short],
  [Fireflies],
  [Critical],
  [Decorum],
  [Rampage],
)

--- grid-consecutive-rows-breaking paged ---
// Test a column that starts overflowing right after another row/column did
// that.
#set page(width: 5cm, height: 2cm)
#grid(
  columns: 4 * (1fr,),
  row-gutter: 10pt,
  column-gutter: (0pt, 10%),
  align(top, image("/assets/images/rhino.png")),
  align(top, rect(inset: 0pt, fill: eastern, align(right)[LoL])),
  [rofl],
  [\ A] * 3,
  [Ha!\ ] * 3,
)

--- grid-same-row-multiple-columns-breaking paged ---
// Test two columns in the same row overflowing by a different amount.
#set page(width: 5cm, height: 2cm)
#grid(
  columns: 3 * (1fr,),
  row-gutter: 8pt,
  column-gutter: (0pt, 10%),
  [A], [B], [C],
  [Ha!\ ] * 6,
  [rofl],
  [\ A] * 3,
  [hello],
  [darkness],
  [my old]
)

--- grid-nested-breaking paged ---
// Test grid within a grid, overflowing.
#set page(width: 5cm, height: 2.25cm)
#grid(
  columns: 4 * (1fr,),
  row-gutter: 10pt,
  column-gutter: (0pt, 10%),
  [A], [B], [C], [D],
  grid(columns: 2, [A], [B], [C\ ]*3, [D]),
  align(top, rect(inset: 0pt, fill: eastern, align(right)[LoL])),
  [rofl],
  [E\ ]*4,
)

--- grid-column-sizing-auto-base paged ---
// Test that auto and relative columns use the correct base.
#grid(
  columns: (auto, 60%),
  rows: (auto, auto),
  rect(width: 50%, height: 0.5cm, fill: conifer),
  rect(width: 100%, height: 0.5cm, fill: eastern),
  rect(width: 50%, height: 0.5cm, fill: forest),
)

--- grid-column-sizing-fr-base paged ---
// Test that fr columns use the correct base.
#grid(
  columns: (1fr,) * 4,
  rows: (1cm,),
  rect(width: 50%, fill: conifer),
  rect(width: 50%, fill: forest),
  rect(width: 50%, fill: conifer),
  rect(width: 50%, fill: forest),
)

--- grid-column-sizing-mixed-base paged ---
// Test that all three kinds of rows use the correct bases.
#set page(height: 4cm, margin: 0cm)
#grid(
  rows: (1cm, 1fr, 1fr, auto),
  rect(height: 50%, width: 100%, fill: conifer),
  rect(height: 50%, width: 100%, fill: forest),
  rect(height: 50%, width: 100%, fill: conifer),
  rect(height: 25%, width: 100%, fill: forest),
)

--- grid-trailing-linebreak-region-overflow paged ---
// Test that trailing linebreak doesn't overflow the region.
#set page(height: 2cm)
#grid[
  Hello \
  Hello \
  Hello \

  World
]

--- grid-breaking-expand-vertically paged ---
// Test that broken cell expands vertically.
#set page(height: 2.25cm)
#grid(
  columns: 2,
  gutter: 10pt,
  align(bottom)[A],
  [
    Top
    #align(bottom)[
      Bottom \
      Bottom

      Top
    ]
  ],
  align(top)[B],
)

--- grid-complete-rows paged ---
// Ensure grids expand enough for the given rows.
#grid(
  columns: (2em, 2em),
  rows: (2em,) * 4,
  fill: red,
  stroke: aqua,
  [a]
)

--- grid-auto-shrink paged ---
// Test iterative auto column shrinking.
#set page(width: 210mm - 2 * 2.5cm + 2 * 10pt)
#set text(11pt)
#table(
  columns: 4,
  [Hello!],
  [Hello there, my friend!],
  [Hello there, my friends! Hi!],
  [Hello there, my friends! Hi! What is going on right now?],
)

--- issue-grid-base-auto-row paged ---
// Test that grid base for auto rows makes sense.
#set page(height: 150pt)
#table(
  columns: (1.5cm, auto),
  rows: (auto, auto),
  rect(width: 100%, fill: red),
  rect(width: 100%, fill: blue),
  rect(width: 100%, height: 50%, fill: green),
)

--- issue-grid-base-auto-row-list paged ---
#rect(width: 100%, height: 1em)
- #rect(width: 100%, height: 1em)
  - #rect(width: 100%, height: 1em)

--- issue-grid-skip paged ---
// Grid now skips a remaining region when one of the cells
// doesn't fit into it at all.
#set page(height: 100pt)
#grid(
  columns: (2cm, auto),
  rows: (auto, auto),
  rect(width: 100%, fill: red),
  rect(width: 100%, fill: blue),
  rect(width: 100%, height: 80%, fill: green),
  [hello \ darkness #parbreak() my \ old \ friend \ I],
  rect(width: 100%, height: 20%, fill: blue),
  polygon(fill: red, (0%, 0%), (100%, 0%), (100%, 20%))
)

--- issue-grid-skip-list paged ---
#set page(height: 60pt)
#lines(2)
- #lines(2)

--- issue-grid-double-skip paged ---
// Ensure that the list does not jump to the third page.
#set page(height: 70pt)
#v(40pt)
The following:
+ A
+ B

--- issue-grid-gutter-skip paged ---
// Ensure gutter rows at the top or bottom of a region are skipped.
#set page(height: 10em)

#table(
  row-gutter: 1.5em,
  inset: 0pt,
  rows: (1fr, auto),
  [a],
  [],
  [],
  [f],
  [e\ e],
  [],
  [a]
)

--- issue-3917-grid-with-infinite-width paged ---
// https://github.com/typst/typst/issues/1918
#set page(width: auto)
#context layout(available => {
  let infinite-length = available.width
  // Error: 3-50 cannot create grid with infinite width
  grid(gutter: infinite-length, columns: 2)[A][B]
})

--- issue-7103-wrong-state-calculation paged ---
#set page(paper: "a10")

#let st = state("st", 0)

#let fn() = {
  st.update(i => i + 1)
  lorem(11)
  st.update(i => i - 1)
}

#grid(fn())

#fn()

Result: #context st.get()
//@@ file tests/suite/layout_grid_headers.typ
--- grid-headers paged pdftags ---
#set pdf(standard: "ua-1")
#set page(width: auto, height: 12em)
#table(
  columns: 5,
  align: center + horizon,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow)
  ),
  ..range(0, 6).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten()
)

--- grid-headers-no-repeat paged ---
// Disable repetition
#set page(width: auto, height: 12em)
#table(
  columns: 5,
  align: center + horizon,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow),
    repeat: false
  ),
  ..range(0, 6).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten()
)

--- grid-headers-gutter paged ---
#set page(width: auto, height: 12em)
#table(
  columns: 5,
  align: center + horizon,
  gutter: 3pt,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow),
  ),
  ..range(0, 6).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten()
)

--- grid-header-relative-row-sizes paged ---
// Relative lengths
#set page(height: 10em)
#table(
  rows: (30%, 30%, auto),
  table.header(
    [*A*],
    [*B*]
  ),
  [C],
  [C]
)

--- grid-header-cell-with-y paged ---
#grid(
  grid.cell(y: 1)[a],
  grid.header(grid.cell(y: 0)[b]),
  grid.cell(y: 2)[c]
)

--- grid-header-cell-with-x paged ---
#grid(
  columns: 2,
  stroke: black,
  inset: 5pt,
  grid.header(grid.cell(x: 0)[b1], grid.cell(x: 0)[b2]),
  // This should skip the header
  grid.cell(x: 1)[c]
)

--- grid-header-last-child paged ---
// When the header is the last grid child, it shouldn't include the gutter row
// after it, because there is none.
#grid(
  columns: 2,
  gutter: 3pt,
  grid.header(
    [a], [b],
    [c], [d]
  )
)

--- grid-header-nested paged ---
#set page(height: 14em)
#let t(n) = table(
  columns: 3,
  align: center + horizon,
  gutter: 3pt,
  table.header(
    table.cell(colspan: 3)[*Cool Zone #n*],
    [*Name*], [*Num*], [*Data*]
  ),
  ..range(0, 5).map(i => ([\##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456])).flatten()
)
#grid(
  gutter: 3pt,
  t(0),
  t(1)
)

--- grid-header-hline-and-vline paged ---
// Test line positioning in header
#table(
  columns: 3,
  stroke: none,
  table.hline(stroke: red, end: 2),
  table.vline(stroke: red, end: 3),
  table.header(
    table.hline(stroke: aqua, start: 2),
    table.vline(stroke: aqua, start: 3), [*A*], table.hline(stroke: orange), table.vline(stroke: orange), [*B*],
    [*C*], [*D*]
  ),
  [a], [b],
  [c], [d],
  [e], [f]
)

--- grid-header-not-at-first-row paged ---
#grid(
  [a],
  grid.header([b])
)

--- grid-header-not-at-first-row-two-columns paged ---
#grid(
  columns: 2,
  [a],
  grid.header([b])
)

--- grid-header-multiple paged ---
#grid(
  grid.header([a]),
  grid.header([b]),
  [a],
)

--- grid-header-multiple-unordered paged ---
#set page(height: 4em)
#grid(
  grid.header(grid.cell(x: 0, y: 4)[y]),
  grid.header([x]),
  [a],
  [b],
  [c],
  [d],
  [e],
  [f],
)

--- grid-header-skip paged ---
#grid(
  columns: 2,
  [x], [y],
  grid.header([a]),
  grid.header([b]),
  grid.cell(x: 1)[c], [d],
  grid.header([e]),
  [f], grid.cell(x: 1)[g]
)

--- grid-header-skip-unordered paged ---
#grid(
  columns: 2,
  [a],
  grid.header(grid.cell(x: 0, y: 2)[y]),
  [b],
  grid.header([x]),
  [c]
)

--- grid-header-rowbreak-auto-pos paged ---
#grid(
  columns: 2,
  [x],
  grid.hline(stroke: red),
  grid.header([a]),
  grid.hline(stroke: 3pt),
  [y],
  grid.header(),
  [z],
)

--- grid-header-rowbreak-fixed-pos paged ---
#grid(
  columns: 2,
  [z],
  grid.hline(stroke: red),
  grid.header(grid.cell(x: 0)[b]),
  grid.hline(stroke: 3pt),
  [w],
  [j],
  grid.header(grid.cell(x: 0, y: 9)[c]),
  [k]
)

--- grid-header-rowbreak-mixed-pos paged ---
#grid(
  columns: 2,
  [a],
  grid.header([x], grid.cell(x: 0)[b]),
  [c],
  grid.hline(stroke: red),
  grid.header([y], grid.cell(x: 0, y: 8)[d]),
  grid.hline(stroke: 3pt),
  [e]
)

--- grid-header-rowbreak-auto-and-fixed-pos paged ---
#grid(
  columns: 2,
  [a],
  grid.header([x]),
  [b],
  grid.header(grid.cell(x: 0, y: 3)[y]),
  [c]
)

--- grid-header-too-large-non-repeating-orphan paged ---
#set page(height: 8em)
#grid(
  grid.header(
    [a\ ] * 5,
    repeat: false,
  ),
  [b]
)

--- grid-header-too-large-repeating-orphan paged ---
#set page(height: 8em)
#grid(
  grid.header(
    [a\ ] * 5,
    repeat: true,
  ),
  [b]
)

--- grid-header-too-large-repeating-orphan-with-footer paged ---
#set page(height: 8em)
#grid(
  grid.header(
    [a\ ] * 5,
    repeat: true,
  ),
  [b],
  grid.footer(
    [c],
    repeat: true,
  )
)

--- grid-header-too-large-repeating-orphan-not-at-first-row paged ---
#set page(height: 8em)
#grid(
  [b],
  grid.header(
    [a\ ] * 5,
    repeat: true,
  ),
  [c],
)

--- table-header-in-grid eval ---
#grid(
  // Error: 3-20 cannot use `table.header` as a grid header
  // Hint: 3-20 use `grid.header` instead
  table.header([a]),
  [a],
)

--- grid-header-in-table eval ---
#table(
  // Error: 3-19 cannot use `grid.header` as a table header
  // Hint: 3-19 use `table.header` instead
  grid.header([a]),
  [a],
)

--- grid-header-in-grid-header eval ---
// Error: 14-28 cannot place a grid header within another header or footer
#grid.header(grid.header[a])

--- table-header-in-grid-header eval ---
// Error: 14-29 cannot place a table header within another header or footer
#grid.header(table.header[a])

--- grid-header-in-table-header eval ---
// Error: 15-29 cannot place a grid header within another header or footer
#table.header(grid.header[a])

--- table-header-in-table-header eval ---
// Error: 15-30 cannot place a table header within another header or footer
#table.header(table.header[a])

--- grid-header-block-with-fixed-height paged ---
#set page(height: 15em)
#table(
  rows: (auto, 2.5em, auto),
  table.header(
    [*Hello*],
    [*World*]
  ),
  block(width: 2em, height: 20em, fill: red)
)

--- grid-header-and-rowspan-non-contiguous-1 paged ---
// Rowspan sizing algorithm doesn't do the best job at non-contiguous content
// ATM.
#set page(height: 15em)

#table(
  rows: (auto, 2.5em, 2em, auto, 5em),
  table.header(
    [*Hello*],
    [*World*]
  ),
  table.cell(rowspan: 3, lines(15))
)

--- grid-header-and-rowspan-non-contiguous-2 paged ---
// Rowspan sizing algorithm doesn't do the best job at non-contiguous content
// ATM.
#set page(height: 15em)

#table(
  rows: (auto, 2.5em, 2em, auto, 5em),
  gutter: 3pt,
  table.header(
    [*Hello*],
    [*World*]
  ),
  table.cell(rowspan: 3, lines(15))
)

--- grid-header-and-rowspan-non-contiguous-3 paged ---
// This should look right
#set page(height: 15em)

#table(
  rows: (auto, 2.5em, 2em, auto),
  gutter: 3pt,
  table.header(
    [*Hello*],
    [*World*]
  ),
  table.cell(rowspan: 3, lines(15))
)

--- grid-header-and-rowspan-contiguous-1 paged ---
// Block should occupy all space
#set page(height: 15em)

#table(
  rows: (auto, 2.5em, 2em, auto),
  gutter: 3pt,
  inset: 0pt,
  table.header(
    [*H*],
    [*W*]
  ),
  table.cell(rowspan: 3, block(height: 2.5em + 2em + 20em, width: 100%, fill: red))
)

--- grid-header-and-rowspan-contiguous-2 paged ---
// Block should occupy all space
#set page(height: 15em)

#table(
  rows: (auto, 2.5em, 10em, 5em, auto),
  gutter: 3pt,
  inset: 0pt,
  table.header(
    [*H*],
    [*W*]
  ),
  table.cell(rowspan: 3, block(height: 2.5em + 2em + 20em, width: 100%, fill: red))
)

--- grid-header-and-large-auto-contiguous paged ---
// Block should occupy all space
#set page(height: 15em)

#table(
  rows: (auto, 4.5em, auto),
  gutter: 3pt,
  inset: 0pt,
  table.header(
    [*H*],
    [*W*]
  ),
  block(height: 2.5em + 2em + 20em, width: 100%, fill: red)
)

--- grid-header-lack-of-space paged ---
// Test lack of space for header + text.
#set page(height: 8em)

#table(
  rows: (auto, 2.5em, auto, auto, 10em),
  gutter: 3pt,
  table.header(
    [*Hello*],
    [*World*]
  ),
  table.cell(rowspan: 3, lorem(80))
)

--- grid-header-orphan-prevention paged ---
// Orphan header prevention test
#set page(height: 12em)
#v(8em)
#grid(
  columns: 3,
  grid.header(
    [*Mui*], [*A*], grid.cell(rowspan: 2, fill: orange)[*B*],
    [*Header*], [*Header* #v(0.1em)]
  ),
  ..([Test], [Test], [Test]) * 20
)

--- grid-header-non-repeating-orphan-prevention paged ---
#set page(height: 5em)
#v(2em)
#grid(
  grid.header(repeat: false)[*Abc*],
  [a],
  [b],
  [c],
  [d]
)

--- grid-header-empty paged ---
// Empty header should just be a repeated blank row
#set page(height: 12em)
#table(
  columns: 4,
  align: center + horizon,
  table.header(),
  ..range(0, 4).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789])).flatten()
)

--- grid-header-containing-rowspan paged ---
// When a header has a rowspan with an empty row, it should be displayed
// properly
#set page(height: 10em)

#let count = counter("g")
#table(
  rows: (auto, 2em, auto, auto),
  table.header(
    [eeec],
    table.cell(rowspan: 2, count.step() + context count.display()),
  ),
  [d],
  block(width: 5em, fill: yellow, lorem(15)),
  [d]
)
#context count.display()

--- grid-header-no-expand-with-col-and-row-pos-cell paged ---
#set page(height: 10em)
#table(
  columns: 2,
  table.header(
    [a], [b],
    [c],
  ),
  // Error: 3-48 cell would conflict with header also spanning row 1
  // Hint: 3-48 try moving the cell or the header
  table.cell(x: 1, y: 1, rowspan: 2, lorem(80))
)

--- grid-header-no-expand-with-row-pos-cell paged ---
#set page(height: 10em)
#table(
  columns: 2,
  table.header(
    [a], [b],
    [c],
  ),
  // Error: 3-42 cell would conflict with header also spanning row 1
  // Hint: 3-42 try moving the cell or the header
  table.cell(y: 1, rowspan: 2, lorem(80))
)

--- grid-nested-with-headers paged ---
// Nested table with header should repeat both headers
#set page(height: 10em)
#table(
  table.header(
    [a]
  ),
  table(
    table.header(
      [b]
    ),
    [a\ b\ c\ d]
  )
)

--- grid-nested-headers paged ---
#set page(height: 12em)
#table(
  table.header(
    table(
      table.header(
        [b]
      ),
      [c],
      [d]
    )
  ),
  [a\ b]
)

--- grid-header-not-at-the-top paged ---
#set page(height: 5em)
#v(2em)
#grid(
  [a],
  [b],
  grid.header[*Abc*],
  [d],
  [e],
  [f],
)

--- grid-header-replace paged ---
#set page(height: 5em)
#v(1.5em)
#grid(
  grid.header[*Abc*],
  [a],
  [b],
  grid.header[*Def*],
  [d],
  [e],
  [f],
)

--- grid-header-replace-orphan paged ---
#set page(height: 5em)
#grid(
  grid.header[*Abc*],
  [a],
  [b],
  grid.header[*Def*],
  [d],
  [e],
  [f],
)

--- grid-header-replace-doesnt-fit paged ---
#set page(height: 5em)
#v(0.8em)
#grid(
  grid.header[*Abc*],
  [a],
  [b],
  grid.header[*Def*],
  [d],
  [e],
  [f],
)

--- grid-header-stroke-edge-cases paged ---
// Test header stroke priority edge case (last header row removed)
#set page(height: 8em)
#table(
  columns: 2,
  stroke: black,
  gutter: (auto, 3pt),
  table.header(
    [c], [d],
  ),
  ..(table.cell(stroke: aqua)[d],) * 8,
)

--- grid-header-hline-bottom paged ---
// Yellow line should be kept here
#set text(6pt)
#table(
  column-gutter: 3pt,
  inset: 1pt,
  table.header(
    [a],
    table.hline(stroke: yellow),
  ),
  table.cell(rowspan: 2)[b]
)

--- grid-header-hline-bottom-manually paged ---
// Red line should be kept here
#set page(height: 6em)
#set text(6pt)
#table(
  column-gutter: 3pt,
  inset: 1pt,
  table.header(
    table.hline(stroke: red, position: bottom),
    [a],
  ),
  [a],
  table.cell(stroke: aqua)[b]
)

--- grid-header-rowspan-base paged ---
#set page(height: 7em)
#set text(6pt)
#let full-block = block(width: 2em, height: 100%, fill: red)
#table(
  columns: 3,
  inset: 1.5pt,
  table.header(
    [a], full-block, table.cell(rowspan: 2, full-block),
    [b]
  )
)

--- grid-header-row-pos-cell-inside-conflicts-with-row-before paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 3,
  inset: 1.5pt,
  table.cell(y: 0)[a],
  table.header(
    table.hline(stroke: red),
    table.hline(y: 1, stroke: aqua),
    // Error: 5-24 cell would cause header to expand to non-empty row 0
    // Hint: 5-24 try moving its cells to available rows
    table.cell(y: 0)[b],
    [c]
  )
)

--- grid-header-row-pos-cell-inside-conflicts-with-row-before-after-first-empty-row paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 3,
  inset: 1.5pt,
  // Rows: Occupied, Empty, Occupied, Empty, Empty, ...
  // Should not be able to expand header from the second Empty to the second Occupied.
  table.cell(y: 0)[a],
  table.cell(y: 2)[a],
  table.header(
    table.hline(stroke: red),
    table.hline(y: 3, stroke: aqua),
    // Error: 5-24 cell would cause header to expand to non-empty row 2
    // Hint: 5-24 try moving its cells to available rows
    table.cell(y: 2)[b],
  )
)

--- grid-header-auto-pos-cell-inside-conflicts-with-row-after paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 2,
  inset: 1.5pt,
  table.cell(y: 1)[a],
  table.header(
    [b], [c],
    // Error: 6-7 cell would cause header to expand to non-empty row 1
    // Hint: 6-7 try moving its cells to available rows
    [d],
  ),
)

--- grid-header-row-pos-cell-inside-conflicts-with-row-after paged ---
#set page(margin: 2pt)
#set text(6pt)
#table(
  columns: 2,
  inset: 1.5pt,
  table.cell(y: 2)[a],
  table.header(
    [b], [c],
    // Error: 5-24 cell would cause header to expand to non-empty row 2
    // Hint: 5-24 try moving its cells to available rows
    table.cell(y: 3)[d],
  ),
)

--- grid-header-collision-multiple-ordered paged ---
#grid(
  columns: 2,
  grid.cell(x: 0, y: 0)[a],
  grid.cell(x: 1, y: 0)[a],
  grid.cell(x: 0, y: 3)[a],
  grid.header(grid.cell(x: 0, y: 2)[y]),
  // Error: 15-39 cell would cause header to expand to non-empty row 3
  // Hint: 15-39 try moving its cells to available rows
  grid.header(grid.cell(x: 0, y: 3)[y]),
  grid.header(grid.cell(x: 0, y: 4)[y]),
)

--- grid-header-collision-multiple-unordered paged ---
#grid(
  columns: 2,
  grid.cell(x: 0, y: 0)[a],
  grid.cell(x: 1, y: 0)[a],
  grid.header(grid.cell(x: 0, y: 2)[y]),
  grid.header(grid.cell(x: 0, y: 3)[y]),
  grid.header(grid.cell(x: 0, y: 4)[y]),
  // Error: 3-27 cell would conflict with header also spanning row 3
  // Hint: 3-27 try moving the cell or the header
  grid.cell(x: 0, y: 3)[a]
)

--- grid-header-collision-multiple-rowspan paged ---
#grid(
  columns: 2,
  grid.cell(x: 0, y: 0)[a],
  grid.cell(x: 1, y: 0)[a],
  grid.header(grid.cell(x: 0, y: 2)[y]),
  grid.header(grid.cell(x: 0, y: 3)[y]),
  grid.header(grid.cell(x: 0, y: 4)[y]),
  // Error: 3-39 cell would conflict with header also spanning row 2
  // Hint: 3-39 try moving the cell or the header
  grid.cell(x: 0, y: 1, rowspan: 2)[a]
)

--- issue-5359-column-override-stays-inside-header paged ---
#table(
  columns: 3,
  [Outside],
  table.header(
    [A], table.cell(x: 1)[B], [C],
    table.cell(x: 1)[D],
  ),
)

--- issue-6666-auto-hlines-around-header paged ---
#table(
	columns: 2,
	table.hline(stroke: 2pt + blue),
	table.header([*foo*], [*bar*]),
	table.hline(stroke: 1.5pt + red),
	table.cell(colspan: 2)[_asdf_],
	table.hline(stroke: 1.5pt + red),
	[a], [b],
	[c], [d],
)
//@@ file tests/suite/layout_grid_html.typ
--- basic-table html ---
#table(
  columns: 3,
  rows: 3,

  table.header(
    [The],
    [first],
    [and],
    [the],
    [second],
    [row],
    table.hline(stroke: red)
  ),

  table.cell(x: 1, rowspan: 2)[Baz],
  [Foo],
  [Bar],

  [1],
  // Baz spans into the next cell
  [2],

  table.cell(colspan: 2)[3],
  [4],

  table.footer(
    [The],
    [last],
    [row],
  ),
)

--- col-gutter-table html ---
#table(
  columns: 3,
  column-gutter: 3pt,
  table.header([a], [b], [c]),
  [d], [e], [f],
  [g], [h], [i]
)

--- row-gutter-table html ---
#table(
  columns: 3,
  row-gutter: 3pt,
  table.header([a], [b], [c]),
  [d], [e], [f],
  [g], [h], [i]
)

--- col-row-gutter-table html ---
#table(
  columns: 3,
  gutter: 3pt,
  table.header([a], [b], [c]),
  [d], [e], [f],
  [g], [h], [i]
)

--- table-wide-header-table html ---
#table(
  columns: 3,
  table.header(
    [a], [b], [c],
    [d], [e], [f],
    [g], [h], [i]
  )
)

--- table-wide-header-gutter-table html ---
#table(
  columns: 3,
  gutter: 3pt,
  table.header(
    [a], [b], [c],
    [d], [e], [f],
    [g], [h], [i]
  )
)

--- multi-header-table html ---
#table(
  columns: 2,

  table.header(
    [First], [Header]
  ),
  table.header(
    [Second], [Header]
  ),
  table.header(
    [Level 2], [Header],
    level: 2,
  ),
  table.header(
    [Level 3], [Header],
    level: 3,
  ),

  [Body], [Cells],
  [Yet], [More],

  table.footer(
    [Footer], [Row],
    [Ending], [Table],
  ),
)

--- multi-header-table-gutter html ---
#table(
  columns: 2,
  gutter: 3pt,
  table.header(
    [First], [Header]
  ),
  table.header(
    [Second], [Header]
  ),
  table.header(
    [Level 2], [Header],
    level: 2,
  ),
  table.header(
    [Level 3], [Header],
    level: 3,
  ),

  [Body], [Cells],
  [Yet], [More],

  table.footer(
    [Footer], [Row],
    [Ending], [Table],
  ),
)

--- multi-header-inside-table html ---
#table(
  columns: 2,

  table.header(
    [First], [Header]
  ),
  table.header(
    [Second], [Header]
  ),
  table.header(
    [Level 2], [Header],
    level: 2,
  ),
  table.header(
    [Level 3], [Header],
    level: 3,
  ),

  [Body], [Cells],
  [Yet], [More],

  table.header(
    [Level 2], [Header Inside],
    level: 2,
  ),
  table.header(
    [Level 3],
    level: 3,
  ),

  [Even], [More],
  [Body], [Cells],

  table.header(
    [One Last Header],
    [For Good Measure],
    repeat: false,
    level: 4,
  ),

  table.footer(
    [Footer], [Row],
    [Ending], [Table],
  ),
)

--- issue-7751-table-cell-show-rules html ---
#show table.cell.where(x: 0): [0]
#show table.cell.where(x: 1): [1]
#show table.cell.where(x: 2): [2]

#table(
  columns: 3,
  rows: 3,

  table.header(
    [Text], [Text], [Text],
    table.hline(stroke: red)
  ),

  [Text],
  [Text],
  [Text],

  table.footer(
    [Text],
    [Text],
    [Text],
  ),
)
//@@ file tests/suite/layout_grid_positioning.typ
// Test cell positioning in grids.

--- grid-cell-show-x-y paged ---
#{
  show grid.cell: it => (it.x, it.y)
  grid(
    columns: 2,
    inset: 5pt,
    fill: aqua,
    gutter: 3pt,
    [Hello], [World],
    [Sweet], [Home]
  )
}
#{
  show table.cell: it => pad(rest: it.inset)[#(it.x, it.y)]
  table(
    columns: 2,
    gutter: 3pt,
    [Hello], [World],
    [Sweet], [Home]
  )
}

--- grid-cell-position-out-of-order paged ---
// Positioning cells in a different order than they appear
#grid(
  columns: 2,
  [A], [B],
  grid.cell(x: 1, y: 2)[C], grid.cell(x: 0, y: 2)[D],
  grid.cell(x: 1, y: 1)[E], grid.cell(x: 0, y: 1)[F],
)

--- grid-cell-position-extra-rows paged ---
// Creating more rows by positioning out of bounds
#grid(
  columns: 3,
  rows: 1.5em,
  inset: 5pt,
  fill: (x, y) => if (x, y) == (0, 0) { blue } else if (x, y) == (2, 3) { red } else { green },
  [A],
  grid.cell(x: 2, y: 3)[B]
)

#table(
  columns: (3em, 1em, 3em),
  rows: 1.5em,
  inset: (top: 0pt, bottom: 0pt, rest: 5pt),
  fill: (x, y) => if (x, y) == (0, 0) { blue } else if (x, y) == (2, 3) { red } else { green },
  align: (x, y) => (left, center, right).at(x),
  [A],
  table.cell(x: 2, y: 3)[B]
)

--- grid-cell-position-collide paged ---
#grid(
  [A],
  // Error: 3-42 attempted to place a second cell at column 0, row 0
  // Hint: 3-42 try specifying your cells in a different order
  grid.cell(x: 0, y: 0)[This shall error]
)

--- table-cell-position-collide paged ---
#table(
  [A],
  // Error: 3-43 attempted to place a second cell at column 0, row 0
  // Hint: 3-43 try specifying your cells in a different order
  table.cell(x: 0, y: 0)[This shall error]
)

--- grid-cell-position-automatic-skip-manual paged ---
// Automatic position cell skips custom position cell
#grid(
  grid.cell(x: 0, y: 0)[This shall not error],
  [A]
)

--- grid-cell-position-x-out-of-bounds paged ---
#grid(
  columns: 2,
  [A],
  // Error: 3-36 cell could not be placed at invalid column 2
  grid.cell(x: 2)[This shall error]
)

--- grid-cell-position-partial paged ---
// Partial positioning
#grid(
  columns: 3,
  rows: 1.5em,
  inset: 5pt,
  fill: aqua,
  [A], grid.cell(y: 1, fill: green)[B], [C], grid.cell(x: auto, y: 1, fill: green)[D], [E],
  grid.cell(y: 2, fill: green)[F], grid.cell(x: 0, fill: orange)[G], grid.cell(x: 0, y: auto, fill: orange)[H],
  grid.cell(x: 1, fill: orange)[I]
)

#table(
  columns: 3,
  rows: 1.5em,
  inset: 5pt,
  fill: aqua,
  [A], table.cell(y: 1, fill: green)[B], [C], table.cell(x: auto, y: 1, fill: green)[D], [E],
  table.cell(y: 2, fill: green)[F], table.cell(x: 0, fill: orange)[G], table.cell(x: 0, y: auto, fill: orange)[H],
  table.cell(x: 1, fill: orange)[I]
)

--- grid-cell-position-partial-collide paged ---
#grid(
  columns: 2,
  [A], [B],
  // Error: 3-21 cell could not be placed in row 0 because it was full
  // Hint: 3-21 try specifying your cells in a different order
  grid.cell(y: 0)[C]
)

--- table-cell-position-partial-collide paged ---
#table(
  columns: 2,
  [A], [B],
  // Error: 3-22 cell could not be placed in row 0 because it was full
  // Hint: 3-22 try specifying your cells in a different order
  table.cell(y: 0)[C]
)

--- grid-calendar paged ---
#set page(width: auto)
#show grid.cell: it => {
  if it.y == 0 {
    set text(white)
    strong(it)
  } else {
    // For the second row and beyond, we will write the day number for each
    // cell.

    // In general, a cell's index is given by cell.x + columns * cell.y.
    // Days start in the second grid row, so we subtract 1 row.
    // But the first day is day 1, not day 0, so we add 1.
    let day = it.x + 7 * (it.y - 1) + 1
    if day <= 31 {
      // Place the day's number at the top left of the cell.
      // Only if the day is valid for this month (not 32 or higher).
      place(top + left, dx: 2pt, dy: 2pt, text(8pt, red.darken(40%))[#day])
    }
    it
  }
}

#grid(
  fill: (x, y) => if y == 0 { gray.darken(50%) },
  columns: (30pt,) * 7,
  rows: (auto, 30pt),
  // Events will be written at the bottom of each day square.
  align: bottom,
  inset: 5pt,
  stroke: (thickness: 0.5pt, dash: "densely-dotted"),

  [Sun], [Mon], [Tue], [Wed], [Thu], [Fri], [Sat],

  // This event will occur on the first Friday (sixth column).
  grid.cell(x: 5, fill: yellow.darken(10%))[Call],

  // This event will occur every Monday (second column).
  // We have to repeat it 5 times so it occurs every week.
  ..(grid.cell(x: 1, fill: red.lighten(50%))[Meet],) * 5,

  // This event will occur at day 19.
  grid.cell(x: 4, y: 3, fill: orange.lighten(25%))[Talk],

  // These events will occur at the second week, where available.
  grid.cell(y: 2, fill: aqua)[Chat],
  grid.cell(y: 2, fill: aqua)[Walk],
)

--- grid-exam paged ---
#set page(width: auto)
#show table.cell: it => {
  if it.x == 0 or it.y == 0 {
    set text(white)
    strong(it)
  } else if it.body == [] {
    // Replace empty cells with 'N/A'
    pad(rest: it.inset)[_N/A_]
  } else {
    it
  }
}

#table(
  fill: (x, y) => if x == 0 or y == 0 { gray.darken(50%) },
  columns: 4,
  [], [Exam 1], [Exam 2], [Exam 3],
  ..([John], [Mary], [Jake], [Robert]).map(table.cell.with(x: 0)),

  // Mary got grade A on Exam 3.
  table.cell(x: 3, y: 2, fill: green)[A],

  // Everyone got grade A on Exam 2.
  ..(table.cell(x: 2, fill: green)[A],) * 4,

  // Robert got grade B on other exams.
  ..(table.cell(y: 4, fill: aqua)[B],) * 2,
)
//@@ file tests/suite/layout_grid_rowspan.typ
--- grid-rowspan paged ---
#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  grid.cell(rowspan: 2, fill: orange)[*Left*],
  [Right A], [Right A], [Right A],
  [Right B], grid.cell(colspan: 2, rowspan: 2, fill: orange.darken(10%))[B Wide],
  [Left A], [Left A],
  [Left B], [Left B], grid.cell(colspan: 2, rowspan: 3, fill: orange)[Wide and Long]
)

#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  table.cell(rowspan: 2, fill: orange)[*Left*],
  [Right A], [Right A], [Right A],
  [Right B], table.cell(colspan: 2, rowspan: 2, fill: orange.darken(10%))[B Wide],
  [Left A], [Left A],
  [Left B], [Left B], table.cell(colspan: 2, rowspan: 3, fill: orange)[Wide and Long]
)

--- grid-rowspan-gutter paged ---
#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  grid.cell(rowspan: 2, fill: orange)[*Left*],
  [Right A], [Right A], [Right A],
  [Right B], grid.cell(colspan: 2, rowspan: 2, fill: orange.darken(10%))[B Wide],
  [Left A], [Left A],
  [Left B], [Left B], grid.cell(colspan: 2, rowspan: 3, fill: orange)[Wide and Long]
)

#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  table.cell(rowspan: 2, fill: orange)[*Left*],
  [Right A], [Right A], [Right A],
  [Right B], table.cell(colspan: 2, rowspan: 2, fill: orange.darken(10%))[B Wide],
  [Left A], [Left A],
  [Left B], [Left B], table.cell(colspan: 2, rowspan: 3, fill: orange)[Wide and Long]
)

--- grid-rowspan-fixed-size paged ---
// Fixed-size rows
#set page(height: 10em)
#grid(
  columns: 2,
  rows: 1.5em,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  grid.cell(rowspan: 3)[R1], [b],
  [c],
  [d],
  [e], [f],
  grid.cell(rowspan: 5)[R2], [h],
  [i],
  [j],
  [k],
  [l],
  [m], [n]
)

--- grid-rowspan-cell-coordinates paged ---
// Cell coordinate tests
#set page(height: 10em)
#show table.cell: it => [(#it.x, #it.y)]
#table(
  columns: 3,
  fill: red,
  [a], [b], table.cell(rowspan: 2)[c],
  table.cell(colspan: 2)[d],
  table.cell(colspan: 3, rowspan: 10)[a],
  table.cell(colspan: 2)[b],
)
#table(
  columns: 3,
  gutter: 3pt,
  fill: red,
  [a], [b], table.cell(rowspan: 2)[c],
  table.cell(colspan: 2)[d],
  table.cell(colspan: 3, rowspan: 9)[a],
  table.cell(colspan: 2)[b],
)

--- grid-rowspan-over-auto-row paged ---
// Auto row expansion
#set page(height: 10em)
#grid(
  columns: (1em, 1em),
  rows: (0.5em, 0.5em, auto),
  fill: orange,
  gutter: 3pt,
  grid.cell(rowspan: 4, [x x x x] + place(bottom)[*Bot*]),
  [a],
  [b],
  [c],
  [d]
)

--- grid-rowspan-excessive paged ---
// Excessive rowspan (no gutter)
#set page(height: 10em)
#table(
  columns: 4,
  fill: red,
  [a], [b], table.cell(rowspan: 2)[c], [d],
  table.cell(colspan: 2, stroke: (bottom: aqua + 2pt))[e], table.cell(stroke: (bottom: aqua))[f],
  table.cell(colspan: 2, rowspan: 10)[R1], table.cell(colspan: 2, rowspan: 10)[R2],
  [b],
)

--- grid-rowspan-excessive-gutter paged ---
// Excessive rowspan (with gutter)
#set page(height: 10em)
#table(
  columns: 4,
  gutter: 3pt,
  fill: red,
  [a], [b], table.cell(rowspan: 2)[c], [d],
  table.cell(colspan: 2, stroke: (bottom: aqua + 2pt))[e], table.cell(stroke: (bottom: aqua))[f],
  table.cell(colspan: 2, rowspan: 10)[R1], table.cell(colspan: 2, rowspan: 10)[R2],
  [b],
)

--- grid-rowspan-over-fr-row-at-end paged ---
// Fractional rows
// They cause the auto row to expand more than needed.
#set page(height: 10em)
#grid(
  fill: red,
  gutter: 3pt,
  columns: 3,
  rows: (1em, auto, 1fr),
  [a], [b], grid.cell(rowspan: 3, block(height: 4em, width: 1em, fill: orange)),
  [c], [d],
  [e], [f]
)

--- grid-rowspan-over-fr-row-at-start paged ---
// Fractional rows
#set page(height: 10em)
#grid(
  fill: red,
  gutter: 3pt,
  columns: 3,
  rows: (1fr, auto, 1em),
  [a], [b], grid.cell(rowspan: 3, block(height: 4em, width: 1em, fill: orange)),
  [c], [d],
  [e], [f]
)

--- grid-rowspan-cell-order paged ---
// Cell order
#let count = counter("count")
#show grid.cell: it => {
  count.step()
  context count.display()
}

#grid(
  columns: (2em,) * 3,
  stroke: aqua,
  rows: 1.2em,
  fill: (x, y) => if calc.odd(x + y) { red } else { orange },
  [a], grid.cell(rowspan: 2)[b], grid.cell(rowspan: 2)[c],
  [d],
  grid.cell(rowspan: 2)[f], [g], [h],
  [i], [j],
  [k], [l], [m],
  grid.cell(rowspan: 2)[n], [o], [p],
  [q], [r],
  [s], [t], [u]
)

--- grid-rowspan-unbreakable-1 paged ---
#table(
    columns: 3,
    rows: (auto, auto, auto, 2em),
    gutter: 3pt,
    table.cell(rowspan: 4)[a \ b\ c\ d\ e], [c], [d],
    [e], table.cell(breakable: false, rowspan: 2)[f],
    [g]
)

--- grid-rowspan-unbreakable-2 paged ---
// Test cell breakability
#show grid.cell: it => {
  test(it.breakable, (it.x, it.y) != (0, 6) and (it.y in (2, 5, 6) or (it.x, it.y) in ((0, 1), (2, 3), (1, 7))))
  it.breakable
}
#grid(
  columns: 3,
  rows: (6pt, 1fr, auto, 1%, 1em, auto, auto, 0.2in),
  row-gutter: (0pt, 0pt, 0pt, auto),
  [a], [b], [c],
  grid.cell(rowspan: 3)[d], [e], [f],
  [g], [h],
  [i], grid.cell(rowspan: 2)[j],
  [k],
  grid.cell(y: 5)[l],
  grid.cell(y: 6, breakable: false)[m], grid.cell(y: 6, breakable: true)[n],
  grid.cell(y: 7, breakable: false)[o], grid.cell(y: 7, breakable: true)[p], grid.cell(y: 7, breakable: auto)[q]
)

--- grid-rowspan-in-all-columns-stroke paged ---
#table(
  columns: 2,
  table.cell(stroke: (bottom: red))[a], [b],
  table.hline(stroke: green),
  table.cell(stroke: (top: yellow, left: green, right: aqua, bottom: blue), colspan: 1, rowspan: 2)[d], table.cell(colspan: 1, rowspan: 2)[e],
  [f],
  [g]
)

--- grid-rowspan-in-all-columns-stroke-gutter paged ---
#table(
  columns: 2,
  gutter: 3pt,
  table.cell(stroke: (bottom: red))[a], [b],
  table.hline(stroke: green),
  table.cell(stroke: (top: yellow, left: green, right: aqua, bottom: blue), colspan: 1, rowspan: 2)[d], table.cell(colspan: 1, rowspan: 2)[e],
  [f],
  [g]
)

--- grid-rowspan-block-full-height paged ---
// Block below shouldn't expand to the end of the page, but stay within its
// rows' boundaries.
#set page(height: 9em)
#table(
  rows: (1em, 1em, 1fr, 1fr, auto),
  table.cell(rowspan: 2, block(width: 2em, height: 100%, fill: red)),
  table.cell(rowspan: 2, block(width: 2em, height: 100%, fill: red)),
  [a]
)

--- grid-rowspan-block-overflow paged ---
#set page(height: 7em)
#table(
  columns: 3,
  [], [], table.cell(breakable: true, rowspan: 2, block(width: 2em, height: 100%, fill: red)),
  table.cell(breakable: false, block(width: 2em, height: 100%, fill: red)),
  table.cell(breakable: false, rowspan: 2, block(width: 2em, height: 100%, fill: red)),
)

// Rowspan split tests

--- grid-rowspan-split-1 paged ---
#set page(height: 10em)
#table(
  columns: 2,
  rows: (auto, auto, 3em),
  fill: red,
  [a], table.cell(rowspan: 3, block(width: 50%, height: 10em, fill: orange) + place(bottom)[*ZD*]),
  [e],
  [f]
)

--- grid-rowspan-split-2 paged ---
#set page(height: 10em)
#table(
  columns: 2,
  rows: (auto, auto, 3em),
  row-gutter: 1em,
  fill: red,
  [a], table.cell(rowspan: 3, block(width: 50%, height: 10em, fill: orange) + place(bottom)[*ZD*]),
  [e],
  [f]
)

--- grid-rowspan-split-3 paged ---
#set page(height: 5em)
#table(
  columns: 2,
  fill: red,
  inset: 0pt,
  table.cell(fill: orange, rowspan: 10, place(bottom)[*Z*] + [x\ ] * 10 + place(bottom)[*ZZ*]),
  ..([y],) * 10,
  [a], [b],
)

--- grid-rowspan-split-4 paged ---
#set page(height: 5em)
#table(
  columns: 2,
  fill: red,
  inset: 0pt,
  gutter: 2pt,
  table.cell(fill: orange, rowspan: 10, place(bottom)[*Z*] + [x\ ] * 10 + place(bottom)[*ZZ*]),
  ..([y],) * 10,
  [a], [b],
)

--- grid-rowspan-split-5 paged ---
#set page(height: 5em)
#table(
  columns: 2,
  fill: red,
  inset: 0pt,
  table.cell(fill: orange, rowspan: 10, breakable: false, place(bottom)[*Z*] + [x\ ] * 10 + place(bottom)[*ZZ*]),
  ..([y],) * 10,
  [a], [b],
)

--- grid-rowspan-split-6 paged ---
#set page(height: 5em)
#table(
  columns: 2,
  fill: red,
  inset: 0pt,
  gutter: 2pt,
  table.cell(fill: orange, rowspan: 10, breakable: false, place(bottom)[*Z*] + [x\ ] * 10 + place(bottom)[*ZZ*]),
  ..([y],) * 10,
  [a], [b],
)

--- grid-rowspan-split-7 paged ---
#set page(height: 5em)
#grid(
  columns: 2,
  stroke: red,
  inset: 5pt,
  grid.cell(rowspan: 5)[a\ b\ c\ d\ e]
)

--- grid-rowspan-split-8 paged ---
#set page(height: 5em)
#table(
  columns: 2,
  gutter: 3pt,
  stroke: red,
  inset: 5pt,
  table.cell(rowspan: 5)[a\ b\ c\ d\ e]
)

// Rowspan split without ending at the auto row

--- grid-rowspan-split-9 paged ---
#set page(height: 6em)
#table(
  rows: (4em,) * 7 + (auto,) + (4em,) * 7,
  columns: 2,
  column-gutter: 1em,
  row-gutter: (1em, 2em) * 4,
  fill: (x, y) => if calc.odd(x + y) { orange.lighten(20%) } else { red },
  table.cell(rowspan: 15, [a \ ] * 15),
  [] * 15
)

--- grid-rowspan-split-10 paged ---
#set page(height: 6em)
#table(
  rows: (4em,) * 7 + (auto,) + (4em,) * 7,
  columns: 2,
  column-gutter: 1em,
  row-gutter: (1em, 2em) * 4,
  fill: (x, y) => if calc.odd(x + y) { green } else { green.darken(40%) },
  table.cell(rowspan: 15, block(fill: blue, width: 2em, height: 4em * 14 + 3em)),
  [] * 15
)

--- grid-rowspan-split-11 paged ---
#set page(height: 6em)
#table(
  rows: (3em,) * 15,
  columns: 2,
  column-gutter: 1em,
  row-gutter: (1em, 2em) * 4,
  fill: (x, y) => if calc.odd(x + y) { aqua } else { blue },
  table.cell(breakable: true, rowspan: 15, [a \ ] * 15),
  [] * 15
)

// Some splitting corner cases

--- grid-rowspan-split-12 paged ---
// Inside the larger rowspan's range, there's an unbreakable rowspan and a
// breakable rowspan. This should work normally.
// The auto row will also expand ignoring the last fractional row.
#set page(height: 10em)
#table(
    gutter: 0.5em,
    columns: 2,
    rows: (2em,) * 10 + (auto, auto, 2em, 1fr),
    fill: (_, y) => if calc.even(y) { aqua } else { blue },
    table.cell(rowspan: 14, block(width: 2em, height: 2em * 10 + 2em + 5em, fill: red)[]),
    ..([a],) * 5,
    table.cell(rowspan: 3)[a\ b],
    table.cell(rowspan: 5, [a\ b\ c\ d\ e\ f\ g\ h]),
    [z]
)

--- grid-rowspan-split-13 paged ---
// Inset moving to next region bug
#set page(width: 10cm, height: 2.5cm, margin: 0.5cm)
#set text(size: 11pt)
#table(
  columns: (1fr, 1fr, 1fr),
  [A],
  [B],
  [C],
  [D],
  table.cell(rowspan: 2, lorem(4)),
  [E],
  [F],
  [G],
)

--- grid-rowspan-split-14 paged ---
// Second lorem must be sent to the next page, too big
#set page(width: 10cm, height: 9cm, margin: 1cm)
#set text(size: 11pt)
#table(
  columns: (1fr, 1fr, 1fr),
  align: center,
  rows: (4cm, auto),
  [A], [B], [C],
  table.cell(rowspan: 4, breakable: false, lorem(10)),
  [D],
  table.cell(rowspan: 2, breakable: false, lorem(20)),
  [E],
)

--- grid-rowspan-split-15 paged ---
// Auto row must expand properly in both cases
#set text(10pt)
#show table.cell: it => if it.x == 0 { it } else { layout(size => size.height) }
#table(
  columns: 2,
  rows: (1em, auto, 2em, 3em, 4em),
  gutter: 3pt,
  table.cell(rowspan: 5, block(fill: orange, height: 15em)[a]),
  [b],
  [c],
  [d],
  [e],
  [f]
)

#table(
  columns: 2,
  rows: (1em, auto, 2em, 3em, 4em),
  gutter: 3pt,
  table.cell(rowspan: 5, breakable: false, block(fill: orange, height: 15em)[a]),
  [b],
  [c],
  [d],
  [e],
  [f]
)

--- grid-rowspan-split-16 paged ---
// Expanding on unbreakable auto row
#set page(height: 7em, margin: (bottom: 2em))
#grid(
  columns: 2,
  rows: (1em, 1em, auto, 1em, 1em, 1em),
  fill: (x, y) => if x == 0 { aqua } else { blue },
  stroke: black,
  gutter: 2pt,
  grid.cell(rowspan: 5, block(height: 10em)[a]),
  [a],
  [b],
  grid.cell(breakable: false, v(3em) + [c]),
  [d],
  [e],
  [f], [g]
)

--- grid-rowspan-split-17 paged ---
#show table.cell.where(x: 0): strong
#show table.cell.where(y: 0): strong
#set page(height: 13em)
#let lets-repeat(thing, n) = ((thing + colbreak(),) * (calc.max(0, n - 1)) + (thing,)).join()
#table(
  columns: 4,
  fill: (x, y) => if x == 0 or y == 0 { gray },
  [], [Test 1], [Test 2], [Test 3],
  table.cell(rowspan: 15, align: horizon, lets-repeat((rotate(-90deg, reflow: true)[*All Tests*]), 3)),
  ..([123], [456], [789]) * 15
)

--- issue-6399-grid-cell-rowspan-set-rule paged ---
#set grid.cell(rowspan: 2)
#grid(columns: 2, [hehe])
//@@ file tests/suite/layout_grid_rtl.typ
// Test RTL grid.

--- list-rtl paged ---
#set text(dir: rtl)
- מימין לשמאל

--- grid-rtl paged ---
#set text(dir: rtl)
#table(columns: 2)[A][B][C][D]

--- grid-rtl-colspan paged ---
// Test interaction between RTL and colspans
#set text(dir: rtl)
#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  grid.cell(colspan: 4)[*Full Header*],
  grid.cell(colspan: 2, fill: orange)[*Half*],
  grid.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], grid.cell(colspan: 3, fill: orange.darken(10%))[6],
  grid.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], grid.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  grid.cell(colspan: 4)[*Full Header*],
  grid.cell(colspan: 2, fill: orange)[*Half*],
  grid.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], grid.cell(colspan: 3, fill: orange.darken(10%))[6],
  grid.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], grid.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

--- grid-rtl-colspan-stroke paged ---
#set text(dir: rtl)
#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  table.cell(colspan: 4)[*Full Header*],
  table.cell(colspan: 2, fill: orange)[*Half*],
  table.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], table.cell(colspan: 3, fill: orange.darken(10%))[6],
  table.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], table.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  table.cell(colspan: 4)[*Full Header*],
  table.cell(colspan: 2, fill: orange)[*Half*],
  table.cell(colspan: 2, fill: orange.darken(10%))[*Half*],
  [*A*], [*B*], [*C*], [*D*],
  [1], [2], [3], [4],
  [5], table.cell(colspan: 3, fill: orange.darken(10%))[6],
  table.cell(colspan: 2, fill: orange)[7], [8], [9],
  [10], table.cell(colspan: 2, fill: orange.darken(10%))[11], [12]
)

--- grid-rtl-multiple-regions paged ---
// Test multiple regions
#set page(height: 5em)
#set text(dir: rtl)
#grid(
  stroke: red,
  fill: aqua,
  columns: 4,
  [a], [b], [c], [d],
  [a], grid.cell(colspan: 2)[e, f, g, h, i], [f],
  [e], [g], grid.cell(colspan: 2)[eee\ e\ e\ e],
  grid.cell(colspan: 4)[eeee e e e]
)

--- grid-rtl-vline-position paged ---
// Test left and right for vlines in RTL
#set text(dir: rtl)
#grid(
  columns: 3,
  inset: 5pt,
  grid.vline(stroke: red, position: left), grid.vline(stroke: green, position: right), [a],
  grid.vline(stroke: red, position: left), grid.vline(stroke: 2pt, position: right), [b],
  grid.vline(stroke: red, position: left), grid.vline(stroke: 2pt, position: right), [c],
  grid.vline(stroke: aqua, position: right)
)

#grid(
  columns: 3,
  inset: 5pt,
  gutter: 3pt,
  grid.vline(stroke: green, position: left), grid.vline(stroke: red, position: right), [a],
  grid.vline(stroke: blue, position: left), grid.vline(stroke: red, position: right), [b],
  grid.vline(stroke: blue, position: left), grid.vline(stroke: red, position: right), [c],
  grid.vline(stroke: 2pt, position: right)
)

#grid(
  columns: 3,
  inset: 5pt,
  grid.vline(stroke: green, position: start), grid.vline(stroke: red, position: end), [a],
  grid.vline(stroke: 2pt, position: start), grid.vline(stroke: red, position: end), [b],
  grid.vline(stroke: 2pt, position: start), grid.vline(stroke: red, position: end), [c],
  grid.vline(stroke: 2pt, position: start)
)

#grid(
  columns: 3,
  inset: 5pt,
  gutter: 3pt,
  grid.vline(stroke: green, position: start), grid.vline(stroke: red, position: end), [a],
  grid.vline(stroke: blue, position: start), grid.vline(stroke: red, position: end), [b],
  grid.vline(stroke: blue, position: start), grid.vline(stroke: red, position: end), [c],
  grid.vline(stroke: 2pt, position: start)
)

--- grid-rtl-vline-out-of-bounds paged ---
#set text(dir: rtl)
#grid(
  // Error: 8-34 cannot place vertical line at the 'end' position of the end border (x = 1)
  // Hint: 8-34 set the line's position to 'start' or place it at a smaller 'x' index
  [a], grid.vline(position: left)
)

--- grid-rtl-complex paged ---
#set text(dir: rtl)

#grid(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  grid.cell(rowspan: 2, fill: orange)[*Left*],
  [Right A], [Right A], [Right A],
  [Right B], grid.cell(colspan: 2, rowspan: 2, fill: orange.darken(10%))[B Wide],
  [Left A], [Left A],
  [Left B], [Left B], grid.cell(colspan: 2, rowspan: 3, fill: orange)[Wide and Long]
)

#table(
  columns: 4,
  fill: (x, y) => if calc.odd(x + y) { blue.lighten(50%) } else { blue.lighten(10%) },
  inset: 5pt,
  align: center,
  gutter: 3pt,
  table.cell(rowspan: 2, fill: orange)[*Left*],
  [Right A], [Right A], [Right A],
  [Right B], table.cell(colspan: 2, rowspan: 2, fill: orange.darken(10%))[B Wide],
  [Left A], [Left A],
  [Left B], [Left B], table.cell(colspan: 2, rowspan: 3, fill: orange)[Wide and Long]
)

--- grid-rtl-rowspan paged ---
#set page(height: 10em)
#set text(dir: rtl)
#table(
  columns: 2,
  rows: (auto, auto, 3em),
  row-gutter: 1em,
  fill: red,
  [a], table.cell(rowspan: 3, block(width: 50%, height: 10em, fill: orange) + place(bottom)[*ZD*]),
  [e],
  [f]
)

--- grid-rtl-header paged ---
// Headers
#set page(height: 15em)
#set text(dir: rtl)
#table(
  columns: 5,
  align: center + horizon,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
    table.cell(stroke: red)[*N1*], table.cell(stroke: aqua)[*N2*], [*D1*], [*D2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow)
  ),
  ..range(0, 10).map(i => ([\##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten()
)

--- grid-rtl-counter paged ---
// Test interaction between RTL and counters
#set text(dir: rtl)
#let test = counter("test")
#grid(
  columns: (1fr, 1fr),
  inset: 5pt,
  align: center,
  [
    a: // should produce 1
    #test.step()
    #context test.get().first()
  ],
  [
    b: // should produce 2
    #test.step()
    #context test.get().first()
  ],
)

--- grid-rtl-rowspan-counter-equal paged ---
// Test interaction between RTL and counters
#set text(dir: rtl)
#let test = counter("test")
#grid(
  columns: (1fr, 1fr),
  inset: 5pt,
  align: center,
  grid.cell(rowspan: 2, [
    a: // should produce 1
    #test.step()
    #context test.get().first()
  ]),
  grid.cell(rowspan: 2, [
    b: // should produce 2
    #test.step()
    #context test.get().first()
  ]),
)

--- grid-rtl-rowspan-counter-unequal-1 paged ---
// Test interaction between RTL and counters
#set text(dir: rtl)
#let test = counter("test")
#grid(
  columns: (1fr, 1fr),
  inset: 5pt,
  align: center,
  grid.cell(rowspan: 5, [
    b: // will produce 2
    #test.step()
    #context test.get().first()
  ]),
  grid.cell(rowspan: 2, [
    a: // will produce 1
    #test.step()
    #context test.get().first()
  ]),
  grid.cell(rowspan: 3, [
    c: // will produce 3
    #test.step()
    #context test.get().first()
  ]),
)

--- grid-rtl-rowspan-counter-unequal-2 paged ---
// Test interaction between RTL and counters
#set text(dir: rtl)
#let test = counter("test")
#grid(
  columns: (1fr, 1fr),
  inset: 5pt,
  align: center,
  grid.cell(rowspan: 2, [
    a: // will produce 1
    #test.step()
    #context test.get().first()
  ]),
  grid.cell(rowspan: 5, [
    b: // will produce 2
    #test.step()
    #context test.get().first()
  ]),
  grid.cell(rowspan: 3, [
    c: // will produce 3
    #test.step()
    #context test.get().first()
  ]),
)

--- grid-rtl-rowspan-counter-mixed-1 paged ---
// Test interaction between RTL and counters
#set text(dir: rtl)
#let test = counter("test")
#grid(
  columns: (1fr, 1fr),
  inset: 5pt,
  align: center,
  [
    a: // will produce 1
    #test.step()
    #context test.get().first()
  ],
  grid.cell(rowspan: 2, [
    b: // will produce 2
    #test.step()
    #context test.get().first()
  ]),
  [
    c: // will produce 3
    #test.step()
    #context test.get().first()
  ],
)

--- grid-rtl-rowspan-counter-mixed-2 paged ---
// Test interaction between RTL and counters
#set text(dir: rtl)
#let test = counter("test")
#grid(
  columns: (1fr, 1fr),
  inset: 5pt,
  align: center,
  grid.cell(rowspan: 2, [
    b: // will produce 2
    #test.step()
    #context test.get().first()
  ]),
  [
    a: // will produce 1
    #test.step()
    #context test.get().first()
  ],
  [
    c: // will produce 3
    #test.step()
    #context test.get().first()
  ]
)
//@@ file tests/suite/layout_grid_stroke.typ
--- grid-stroke-tiling paged ---
#let double-line = tiling(size: (1.5pt, 1.5pt), {
  place(line(stroke: .6pt, start: (0%, 50%), end: (100%, 50%)))
})

#table(
  stroke: (_, y) => if y != 1 { (bottom: black) },
  columns: 3,
  table.cell(colspan: 3, align: center)[*Epic Table*],
  align(center)[*Name*], align(center)[*Age*], align(center)[*Data*],
  table.hline(stroke: (paint: double-line, thickness: 2pt)),
  [John], [30], [None],
  [Martha], [20], [A],
  [Joseph], [35], [D]
)

--- grid-stroke-folding paged ---
// Test folding
#set grid(stroke: red)
#set grid(stroke: 5pt)

#grid(
  inset: 10pt,
  columns: 2,
  stroke: stroke(dash: "loosely-dotted"),
  grid.vline(start: 2, end: 3, stroke: (paint: green, dash: none)),
  [a], [b],
  grid.hline(end: 1, stroke: blue),
  [c], [d],
  [e], grid.cell(stroke: aqua)[f]
)

--- grid-stroke-set-on-cell-and-line paged ---
// Test set rules on cells and folding
#set table.cell(stroke: 4pt)
#set table.cell(stroke: blue)
#set table.hline(stroke: red)
#set table.hline(stroke: 0.75pt)
#set table.vline(stroke: 0.75pt)
#set table.vline(stroke: aqua)

#table(
  columns: 3,
  gutter: 3pt,
  inset: 5pt,
  [a], [b], table.vline(position: end), [c],
  [d], [e], [f],
  table.hline(position: bottom),
  [g], [h], [i],
)

--- grid-stroke-field-in-show paged ---
// Test stroke field on cell show rules
#set grid.cell(stroke: (x: 4pt))
#set grid.cell(stroke: (x: blue))
#show grid.cell: it => {
  test(it.stroke, (left: stroke(paint: blue, thickness: 4pt, dash: "loosely-dotted"), right: blue + 4pt, top: stroke(thickness: 1pt), bottom: none))
  it
}
#grid(
  stroke: (left: (dash: "loosely-dotted")),
  inset: 5pt,
  grid.hline(stroke: red),
  grid.cell(stroke: (top: 1pt))[a], grid.vline(stroke: yellow),
)

--- grid-stroke-complex paged ---
#table(
  columns: 3,
  [a], table.cell(colspan: 2)[b c],
  table.cell(stroke: blue)[d], [e], [f],
  [g], [h], table.cell(stroke: (left: yellow, top: green, right: aqua, bottom: red))[i],
  [j], [k], [l],
  table.cell(stroke: 3pt)[m], [n], table.cell(stroke: (dash: "loosely-dotted"))[o],
)

--- grid-stroke-array paged ---
// Test per-column stroke array
#let t = table(
  columns: 3,
  stroke: (red, blue, green),
  [a], [b], [c],
  [d], [e], [f],
  [h], [i], [j],
)
#t
#set text(dir: rtl)
#t

--- grid-stroke-func paged ---
#grid(
  columns: 3,
  inset: 3pt,
  stroke: (x, _) => (right: (5pt, (dash: "dotted")).at(calc.rem(x, 2)), bottom: (dash: "densely-dotted")),
  grid.vline(x: 0, stroke: red),
  grid.vline(x: 1, stroke: red),
  grid.vline(x: 2, stroke: red),
  grid.vline(x: 3, stroke: red),
  grid.hline(y: 0, end: 1, stroke: blue),
  grid.hline(y: 1, end: 1, stroke: blue),
  grid.cell[a],
  [b], [c]
)

--- grid-stroke-manually-positioned-lines paged ---
#set page(height: 5em)
#table(
  columns: 3,
  inset: 3pt,
  table.hline(y: 0, end: none, stroke: 3pt + blue),
  table.vline(x: 0, end: none, stroke: 3pt + green),
  table.hline(y: 5, end: none, stroke: 3pt + red),
  table.vline(x: 3, end: none, stroke: 3pt + yellow),
  [a], [b], [c],
  [a], [b], [c],
  [a], [b], [c],
  [a], [b], [c],
  [a], [b], [c],
)

--- grid-stroke-automatically-positioned-lines paged ---
// Automatically positioned lines
// Plus stroke thickness ordering
#table(
  columns: 3,
  table.hline(stroke: red + 5pt),
  table.vline(stroke: blue + 5pt),
  table.vline(stroke: 2pt),
  [a],
  table.vline(x: 1, stroke: aqua + 5pt),
  [b],
  table.vline(stroke: aqua + 5pt),
  [c],
  table.vline(stroke: yellow + 5.2pt),
  table.hline(stroke: green + 5pt),
  [a], [b], [c],
  [a], table.hline(stroke: green + 2pt), table.vline(stroke: 2pt), [b], [c],
)

--- grid-stroke-priority-line paged ---
// Line specification order priority
// The last line should be blue, not red.
// The middle aqua line should be gone due to the 'none' override.
#grid(
  columns: 2,
  inset: 2pt,
  grid.hline(y: 2, stroke: red + 5pt),
  grid.vline(),
  [a], [b],
  grid.hline(stroke: red),
  grid.hline(stroke: none),
  [c], grid.cell(stroke: (top: aqua))[d],
  grid.hline(stroke: blue),
)

--- grid-stroke-hline-position-bottom-gutter paged ---
// Position: bottom and position: end with gutter should have a visible effect
// of moving the lines after the next track.
#table(
  columns: 3,
  gutter: 3pt,
  stroke: blue,
  table.hline(end: 2, stroke: red),
  table.hline(end: 2, stroke: aqua, position: bottom),
  table.vline(end: 2, stroke: green), [a], table.vline(end: 2, stroke: green), table.vline(end: 2, position: end, stroke: orange), [b], table.vline(end: 2, stroke: aqua, position: end), table.vline(end: 2, stroke: green), [c], table.vline(end: 2, stroke: green),
  [d], [e], [f],
  table.hline(end: 2, stroke: red),
  [g], [h], [ie],
  table.hline(end: 2, stroke: green),
)

--- grid-stroke-hline-position-bottom paged ---
// Using position: bottom and position: end without gutter should be the same
// as placing a line after the next track.
#table(
  columns: 3,
  stroke: blue,
  table.hline(end: 2, stroke: red),
  table.hline(end: 2, stroke: aqua, position: bottom),
  table.vline(end: 2, stroke: green), [a], table.vline(end: 2, stroke: green), [b], table.vline(end: 2, stroke: aqua, position: end), table.vline(end: 2, stroke: green), [c], table.vline(end: 2, stroke: green),
  table.hline(end: 2, stroke: 5pt),
  [d], [e], [f],
  table.hline(end: 2, stroke: red),
  [g], [h], [i],
  table.hline(end: 2, stroke: red),
)

--- grid-stroke-vline-position-left-and-right paged ---
// Test left and right for grid vlines.
#grid(
  columns: 3,
  inset: 5pt,
  grid.vline(stroke: green, position: left), grid.vline(stroke: red, position: right), [a],
  grid.vline(stroke: 2pt, position: left), grid.vline(stroke: red, position: right), [b],
  grid.vline(stroke: 2pt, position: left), grid.vline(stroke: red, position: right), [c],
  grid.vline(stroke: 2pt, position: left)
)

#grid(
  columns: 3,
  inset: 5pt,
  gutter: 3pt,
  grid.vline(stroke: green, position: left), grid.vline(stroke: red, position: right), [a],
  grid.vline(stroke: blue, position: left), grid.vline(stroke: red, position: right), [b],
  grid.vline(stroke: blue, position: left), grid.vline(stroke: red, position: right), [c],
  grid.vline(stroke: 2pt, position: left)
)

--- table-stroke-vline-position-left-and-right paged ---
// Test left and right for table vlines.
#table(
  columns: 3,
  inset: 5pt,
  table.vline(stroke: green, position: left), table.vline(stroke: red, position: right), [a],
  table.vline(stroke: 2pt, position: left), table.vline(stroke: red, position: right), [b],
  table.vline(stroke: 2pt, position: left), table.vline(stroke: red, position: right), [c],
  table.vline(stroke: 2pt, position: left)
)

#table(
  columns: 3,
  inset: 5pt,
  gutter: 3pt,
  table.vline(stroke: green, position: left), table.vline(stroke: red, position: right), [a],
  table.vline(stroke: blue, position: left), table.vline(stroke: red, position: right), [b],
  table.vline(stroke: blue, position: left), table.vline(stroke: red, position: right), [c],
  table.vline(stroke: 2pt, position: left)
)

--- grid-stroke-priority-line-cell paged ---
// Hlines and vlines should always appear on top of cell strokes.
#table(
  columns: 3,
  stroke: aqua,
  table.vline(stroke: red, position: end), [a], table.vline(stroke: red), [b], [c],
  table.cell(stroke: blue)[d], [e], [f],
  table.hline(stroke: red),
  [g], table.cell(stroke: blue)[h], [i],
)

#table(
  columns: 3,
  gutter: 3pt,
  stroke: aqua,
  table.vline(stroke: red, position: end), [a], table.vline(stroke: red), [b], [c],
  table.cell(stroke: blue)[d], [e], [f],
  table.hline(stroke: red),
  [g], table.cell(stroke: blue)[h], [i],
)

--- grid-stroke-priority-cell paged ---
// Ensure cell stroke overrides always appear on top.
#table(
  columns: 2,
  stroke: black,
  table.cell(stroke: red)[a], [b],
  [c], [d],
)

#table(
  columns: 2,
  table.cell(stroke: red)[a], [b],
  [c], [d],
)

--- grid-stroke-hline-position-bad paged ---
#table(
  columns: 2,
  [a], [b],
  [c], [d],
  table.hline(stroke: aqua),
  table.hline(position: top),
  // Error: 3-32 cannot place horizontal line at the 'bottom' position of the bottom border (y = 2)
  // Hint: 3-32 set the line's position to 'top' or place it at a smaller 'y' index
  table.hline(position: bottom)
)

--- grid-stroke-border-partial paged ---
// Test partial border line overrides
#set page(width: auto, height: 7em, margin: (bottom: 1em))
#table(
  columns: 4,
  stroke: (x, y) => if y == 0 or y == 4 { orange } else { aqua },
  table.hline(stroke: blue, start: 1, end: 2), table.cell(stroke: red, v(3em)), table.cell(stroke: blue)[b], table.cell(stroke: green)[c], [M],
  [a], [b], [c], [M],
  [d], [e], [f], [M],
  [g], [h], [i], [M],
  table.cell(stroke: red)[a], table.cell(stroke: blue)[b], table.cell(stroke: green)[c], [M],
  table.hline(stroke: blue, start: 1, end: 2),
)

--- grid-stroke-vline-colspan paged ---
// - Vline should be placed after the colspan.
// - Hline should be placed under the full-width rowspan.
#table(
  columns: 3,
  rows: 1.25em,
  inset: 1pt,
  stroke: none,
  table.cell(colspan: 2)[a], table.vline(stroke: red), table.hline(stroke: blue), [b],
  [c], [d], [e],
  table.cell(colspan: 3, rowspan: 2)[a], table.vline(stroke: blue), table.hline(stroke: red)
)

--- grid-stroke-hline-rowspan paged ---
// Red line should be above [c] (hline skips the shortest rowspan).
#set text(6pt)
#table(
  rows: 1em,
  columns: 2,
  inset: 1.5pt,
  table.cell(rowspan: 3)[a], table.cell(rowspan: 2)[b],
  table.hline(stroke: red),
  [c]
)

--- grid-stroke-hline-position-bottom-out-of-bounds paged ---
#table(
  columns: 2,
  gutter: 3pt,
  [a], [b],
  [c], [d], table.vline(stroke: red),
  table.hline(stroke: aqua),
  table.hline(position: top),
  // Error: 3-32 cannot place horizontal line at the 'bottom' position of the bottom border (y = 2)
  // Hint: 3-32 set the line's position to 'top' or place it at a smaller 'y' index
  table.hline(position: bottom)
)

--- grid-stroke-vline-position-bottom-out-of-bounds paged ---
#grid(
  columns: 2,
  [a], [b],
  grid.vline(stroke: aqua),
  grid.vline(position: start),
  // Error: 3-28 cannot place vertical line at the 'end' position of the end border (x = 2)
  // Hint: 3-28 set the line's position to 'start' or place it at a smaller 'x' index
  grid.vline(position: end)
)

--- grid-stroke-vline-position-bottom-out-of-bounds-gutter paged ---
#grid(
  columns: 2,
  gutter: 3pt,
  [a], [b],
  grid.vline(stroke: aqua),
  grid.vline(position: start),
  // Error: 3-28 cannot place vertical line at the 'end' position of the end border (x = 2)
  // Hint: 3-28 set the line's position to 'start' or place it at a smaller 'x' index
  grid.vline(position: end)
)

--- grid-stroke-hline-out-of-bounds paged ---
#grid(
  [a],
  [b],
  // Error: 3-19 cannot place horizontal line at invalid row 3
  grid.hline(y: 3)
)

--- grid-stroke-hline-out-of-bounds-gutter paged ---
#grid(
  gutter: 3pt,
  [a],
  [b],
  // Error: 3-19 cannot place horizontal line at invalid row 3
  grid.hline(y: 3)
)

--- grid-stroke-vline-out-of-bounds paged ---
#table(
  columns: 2,
  [a], [b],
  // Error: 3-20 cannot place vertical line at invalid column 3
  table.vline(x: 3)
)

--- grid-stroke-vline-out-of-bounds-gutter paged ---
#table(
  columns: 2,
  gutter: 3pt,
  [a], [b],
  // Error: 3-20 cannot place vertical line at invalid column 3
  table.vline(x: 3)
)

--- table-hline-in-grid eval ---
// Error: 7-20 cannot use `table.hline` as a grid line
// Hint: 7-20 use `grid.hline` instead
#grid(table.hline())

--- table-vline-in-grid eval ---
// Error: 7-20 cannot use `table.vline` as a grid line
// Hint: 7-20 use `grid.vline` instead
#grid(table.vline())

--- grid-hline-in-table eval ---
// Error: 8-20 cannot use `grid.hline` as a table line
// Hint: 8-20 use `table.hline` instead
#table(grid.hline())

--- grid-vline-in-table eval ---
// Error: 8-20 cannot use `grid.vline` as a table line
// Hint: 8-20 use `table.vline` instead
#table(grid.vline())

--- grid-hline-end-before-start-1 paged ---
#grid(
  columns: 3,
  // Error: 3-31 line cannot end before it starts
  grid.hline(start: 2, end: 1),
  [a], [b], [c],
)

--- grid-hline-end-before-start-2 paged ---
#table(
  columns: 3,
  // Error: 3-32 line cannot end before it starts
  table.vline(start: 2, end: 1),
  [a], [b], [c],
  [d], [e], [f],
  [g], [h], [i],
)

--- grid-hline-position-horizon eval ---
// Error: 24-31 expected `top` or `bottom`, found horizon
#table.hline(position: horizon)

--- grid-vline-position-center eval ---
// Error: 24-30 expected `start`, `left`, `right`, or `end`, found center
#table.vline(position: center)

--- grid-hline-position-right eval ---
// Error: 24-29 expected `top` or `bottom`, found right
#table.hline(position: right)

--- grid-vline-position-top eval ---
// Error: 24-27 expected `start`, `left`, `right`, or `end`, found top
#table.vline(position: top)

--- issue-7398-grid-line-end-oob paged ---
#set page(width: auto)
#table(
  columns: 2,
  [A], [B],
  [C], [D],
  table.vline(end: 3),
  table.hline(end: 3),
)
//@@ file tests/suite/layout_grid_styling.typ
// Test grid styling options.

--- grid-fill-func paged ---
#set page(height: 70pt)
#set grid(fill: (x, y) => if calc.even(x + y) { rgb("aaa") })

#grid(
  columns: (1fr,) * 3,
  stroke: 2pt + rgb("333"),
  [A], [B], [C], [], [], [D \ E \ F \ \ \ G], [H],
)

--- grid-stroke-none paged ---
#grid(columns: 3, stroke: none, fill: green, [A], [B], [C])

--- grid-align paged ---
// Test general alignment.
#grid(
  columns: 3,
  align: left,
  [Hello], [Hello], [Hello],
  [A], [B], [C],
)

// Test alignment with a function.
#grid(
  columns: 3,
  align: (x, y) => (left, center, right).at(x),
  [Hello], [Hello], [Hello],
  [A], [B], [C],
)

// Test alignment with array.
#grid(
  columns: (1fr, 1fr, 1fr),
  align: (left, center, right),
  [A], [B], [C]
)

// Test empty array.
#set align(center)
#grid(
  columns: (1fr, 1fr, 1fr),
  align: (),
  [A], [B], [C]
)

a

--- grid-inset paged ---
// Test inset.
#grid(
  columns: (1fr,) * 3,
  stroke: 2pt + rgb("333"),
  inset: 5pt,
  [A], [B], [C], [], [], [D \ E \ F \ \ \ G], [H],
)

#grid(
  columns: 3,
  inset: 10pt,
  fill: blue,
  [A], [B], [C]
)

#grid(
  columns: 3,
  inset: (y: 10pt),
  [A], [B], [C]
)

#grid(
  columns: 3,
  inset: (left: 20pt, rest: 10pt),
  stroke: 3pt + red,
  [A], [B], [C]
)

#grid(
  columns: 2,
  inset: (
    left: 20pt,
    right: 5pt,
    top: 10pt,
    bottom: 3pt,
  ),
  [A],
  [B],
)

#grid(
  columns: 3,
  fill: (x, y) => (if y == 0 { aqua } else { orange }).darken(x * 15%),
  inset: (x, y) => (left: if x == 0 { 0pt } else { 5pt }, right: if x == 0 { 5pt } else { 0pt }, y: if y == 0 { 0pt } else { 5pt }),
  [A], [B], [C],
  [A], [B], [C],
)

#grid(
  columns: 3,
  inset: (0pt, 5pt, 10pt),
  fill: (x, _) => aqua.darken(x * 15%),
  [A], [B], [C],
)

--- grid-inset-func-panic-trace paged trace ---
#let compute-inset(x, y) = "100"

// Error: 18-31 expected relative length or dictionary, found string
#set grid(inset: compute-inset)

// Trace: 1:2-5:2 (1) while processing grid element
#grid(
  columns: 2,
  [A], [B],
  [C], [D],
)

--- grid-inset-folding paged ---
// Test inset folding
#set grid(inset: 10pt)
#set grid(inset: (left: 0pt))

#grid(
  fill: red,
  inset: (right: 0pt),
  grid.cell(inset: (top: 0pt))[a]
)

--- grid-funcs-gutter paged ---
// Test interaction with gutters.
#grid(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

#grid(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  row-gutter: 5pt,
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

#grid(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  column-gutter: 5pt,
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

#grid(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  gutter: 5pt,
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)
//@@ file tests/suite/layout_grid_subheaders.typ
--- grid-subheaders-demo paged ---
#set page(height: 15.2em)
#table(
  columns: 2,
  align: center,
  table.header(
    table.cell(colspan: 2)[*Regional User Data*],
  ),
  table.header(
    level: 2,
    table.cell(colspan: 2)[*Germany*],
    [*Username*], [*Joined*]
  ),
  [john123], [2024],
  [rob8], [2025],
  [joe1], [2025],
  [joe2], [2025],
  [martha], [2025],
  [pear], [2025],
  table.header(
    level: 2,
    table.cell(colspan: 2)[*United States*],
    [*Username*], [*Joined*]
  ),
  [cool4], [2023],
  [roger], [2023],
  [bigfan55], [2022]
)

--- grid-subheaders-colorful paged ---
#set page(width: auto, height: 12em)
#let rows(n) = {
  range(n).map(i => ([John \##i], table.cell(stroke: green)[123], table.cell(stroke: blue)[456], [789], [?], table.hline(start: 4, end: 5, stroke: red))).flatten()
}
#table(
  columns: 5,
  align: center + horizon,
  table.header(
    table.cell(colspan: 5)[*Cool Zone*],
  ),
  table.header(
    level: 2,
    table.cell(stroke: red)[*Name*], table.cell(stroke: aqua)[*Number*], [*Data 1*], [*Data 2*], [*Etc*],
    table.hline(start: 2, end: 3, stroke: yellow)
  ),
  ..rows(2),
  table.header(
    level: 2,
    table.cell(stroke: red)[*New Name*], table.cell(stroke: aqua, colspan: 4)[*Other Data*],
    table.hline(start: 2, end: 3, stroke: yellow)
  ),
  ..rows(3)
)

--- grid-subheaders-basic paged ---
#grid(
  grid.header([a]),
  grid.header(level: 2, [b]),
  [c]
)

--- grid-subheaders-basic-non-consecutive paged ---
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
)

--- grid-subheaders-basic-replace paged ---
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.header(level: 2, [c]),
  [z],
)

--- grid-subheaders-basic-with-footer paged ---
#grid(
  grid.header([a]),
  grid.header(level: 2, [b]),
  [c],
  grid.footer([d])
)

--- grid-subheaders-basic-non-consecutive-with-footer paged ---
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.footer([f])
)

--- grid-subheaders-repeat paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  grid.header(level: 2, [b]),
  ..([c],) * 10,
)

--- grid-subheaders-repeat-non-consecutive paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 10,
)

--- grid-subheaders-repeat-with-footer paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [m],
  grid.header(level: 2, [b]),
  ..([c],) * 10,
  grid.footer([f])
)

--- grid-subheaders-repeat-gutter paged ---
// Gutter below the header is also repeated
#set page(height: 8em)
#grid(
  inset: (bottom: 0.5pt),
  stroke: (bottom: 1pt),
  gutter: (1pt, 6pt, 1pt),
  grid.header([a]),
  grid.header(level: 2, [b]),
  ..([c],) * 10,
)

--- grid-subheaders-repeat-replace paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 10,
  grid.header(level: 2, [c]),
  ..([z],) * 10,
)

--- grid-subheaders-repeat-replace-multiple-levels paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  grid.header(level: 3, [c]),
  ..([y],) * 10,
  grid.header(level: 2, [d]),
  ..([z],) * 6,
)

--- grid-subheaders-repeat-replace-gutter paged ---
#set page(height: 8em)
#grid(
  gutter: 3pt,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 8,
  grid.header(level: 2, [c]),
  ..([z],) * 4,
)

--- grid-subheaders-repeat-replace-orphan paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 12,
  grid.header(level: 2, [c]),
  ..([z],) * 10,
)

--- grid-subheaders-repeat-replace-double-orphan paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 11,
  grid.header(level: 2, [c]),
  grid.header(level: 3, [d]),
  ..([z],) * 10,
)

--- grid-subheaders-repeat-replace-gutter-orphan-at-child paged ---
#set page(height: 8em)
#grid(
  gutter: 3pt,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 9,
  grid.header(level: 2, [c]),
  [z \ z],
  ..([z],) * 3,
)

--- grid-subheaders-repeat-replace-gutter-orphan-at-gutter paged ---
#set page(height: 8em)
#grid(
  gutter: 3pt,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 9,
  box(height: 3pt),
  grid.header(level: 2, [c]),
  ..([z],) * 4,
)

--- grid-subheaders-repeat-replace-didnt-fit-once paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 10,
  grid.header(level: 2, [c\ c\ c]),
  ..([z],) * 4,
)

--- grid-subheaders-repeat-replace-with-footer paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  grid.header(level: 3, [c]),
  ..([y],) * 10,
  grid.header(level: 2, [d]),
  ..([z],) * 6,
  grid.footer([f])
)

--- grid-subheaders-repeat-replace-with-footer-orphan paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 10,
  grid.header(level: 2, [c]),
  ..([z],) * 10,
  grid.footer([f])
)

--- grid-subheaders-repeat-replace-short-lived paged ---
// No orphan prevention for short-lived headers
// (followed by replacing headers).
#set page(height: 8em)
#grid(
  grid.header([a]),
  grid.header(level: 2, [b]),
  grid.header(level: 2, [c]),
  grid.header(level: 2, [d]),
  grid.header(level: 2, [e]),
  grid.header(level: 2, [f]),
  grid.header(level: 2, [g]),
  grid.header(level: 2, [h]),
  grid.header(level: 2, [i]),
  grid.header(level: 2, [j]),
  grid.header(level: 3, [k]),
  ..([z],) * 10,
)

--- grid-subheaders-repeat-short-lived-also-replaces paged ---
// Short-lived subheaders must still replace their conflicting predecessors.
#set page(height: 8em)
#grid(
  // This has to go
  grid.header(level: 3, [a]),
  [w],
  grid.header(level: 2, [b]),
  grid.header(level: 2, [c]),
  grid.header(level: 2, [d]),
  grid.header(level: 2, [e]),
  grid.header(level: 2, [f]),
  grid.header(level: 2, [g]),
  grid.header(level: 2, [h]),
  grid.header(level: 2, [i]),
  grid.header(level: 2, [j]),
  grid.header(level: 3, [k]),
  ..([z],) * 10,
)

--- grid-subheaders-multi-page-row paged ---
#set page(height: 8em)
#grid(
  columns: 2,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.header(level: 3, [c]),
  [a], [b],
  grid.cell(
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [y],
  ..([z],) * 10,
)

--- grid-subheaders-non-repeat paged ---
#set page(height: 8em)
#grid(
  grid.header(repeat: false, [a]),
  [x],
  grid.header(level: 2, repeat: false, [b]),
  ..([y],) * 10,
)

--- grid-subheaders-non-repeat-replace paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  grid.header(level: 3, [c]),
  ..([y],) * 9,
  grid.header(level: 2, repeat: false, [d]),
  ..([z],) * 6,
)

--- grid-subheaders-non-repeating-replace-orphan paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 12,
  grid.header(level: 2, repeat: false, [c]),
  ..([z],) * 10,
)

--- grid-subheaders-non-repeating-replace-didnt-fit-once paged ---
#set page(height: 8em)
#grid(
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  ..([y],) * 10,
  grid.header(level: 2, repeat: false, [c\ c\ c]),
  ..([z],) * 4,
)

--- grid-subheaders-multi-page-rowspan paged ---
#set page(height: 8em)
#grid(
  columns: 2,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.header(level: 3, [c]),
  [z], [z],
  grid.cell(
    rowspan: 5,
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [cell],
  [cell]
)

--- grid-subheaders-multi-page-row-right-after paged ---
#set page(height: 8em)
#grid(
  columns: 1,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  grid.header(level: 3, [c]),
  grid.cell(
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [done.],
  [done.]
)

--- grid-subheaders-multi-page-rowspan-right-after paged ---
#set page(height: 8em)
#grid(
  columns: 2,
  grid.header([a]),
  [x], [y],
  grid.header(level: 2, [b]),
  grid.header(level: 3, [c]),
  grid.cell(
    rowspan: 5,
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [cell],
  [cell],
  grid.cell(x: 0)[done.],
  grid.cell(x: 0)[done.]
)

--- grid-subheaders-multi-page-row-with-footer paged ---
#set page(height: 8em)
#grid(
  columns: 2,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.header(level: 3, [c]),
  [a], [b],
  grid.cell(
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [y],
  ..([z],) * 10,
  grid.footer([f])
)

--- grid-subheaders-multi-page-rowspan-with-footer paged ---
#set page(height: 8em)
#grid(
  columns: 2,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.header(level: 3, [c]),
  [z], [z],
  grid.cell(
    rowspan: 5,
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [cell],
  [cell],
  grid.footer([f])
)

--- grid-subheaders-multi-page-row-right-after-with-footer paged ---
#set page(height: 8em)
#grid(
  columns: 1,
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  grid.header(level: 3, [c]),
  grid.cell(
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [done.],
  [done.],
  grid.footer([f])
)

--- grid-subheaders-multi-page-rowspan-gutter paged ---
#set page(height: 9em)
#grid(
  columns: 2,
  column-gutter: 4pt,
  row-gutter: (0pt, 4pt, 8pt, 4pt),
  inset: (bottom: 0.5pt),
  stroke: (bottom: 1pt),
  grid.header([a]),
  [x],
  grid.header(level: 2, [b]),
  [y],
  grid.header(level: 3, [c]),
  [z], [z],
  grid.cell(
    rowspan: 5,
    block(fill: red, width: 1.5em, height: 6.4em)
  ),
  [cell],
  [cell],
  [a\ b],
  grid.cell(x: 0)[end],
)

--- grid-subheaders-non-repeating-header-before-multi-page-row paged ---
#set page(height: 6em)
#grid(
  grid.header(repeat: false, [h]),
  [row #colbreak() row]
)


--- grid-subheaders-short-lived-no-orphan-prevention paged ---
// No orphan prevention for short-lived headers.
#set page(height: 8em)
#v(5em)
#grid(
  grid.header(level: 2, [b]),
  grid.header(level: 2, [c]),
  [d]
)

--- grid-subheaders-repeating-orphan-prevention paged ---
#set page(height: 8em)
#v(4.5em)
#grid(
  grid.header(repeat: true, level: 2, [L2]),
  grid.header(repeat: true, level: 4, [L4]),
  [a]
)

--- grid-subheaders-non-repeating-orphan-prevention paged ---
#set page(height: 8em)
#v(4.5em)
#grid(
  grid.header(repeat: false, level: 2, [L2]),
  grid.header(repeat: false, level: 4, [L4]),
  [a]
)

--- grid-subheaders-alone paged ---
#table(
  table.header([a]),
  table.header(level: 2, [b]),
)

--- grid-subheaders-alone-no-orphan-prevention paged ---
#set page(height: 5.3em)
#v(2em)
#grid(
  grid.header([L1]),
  grid.header(level: 2, [L2]),
)

--- grid-subheaders-alone-with-gutter-no-orphan-prevention paged ---
#set page(height: 5.3em)
#v(2em)
#grid(
  gutter: 3pt,
  grid.header([L1]),
  grid.header(level: 2, [L2]),
)

--- grid-subheaders-alone-with-footer paged ---
#table(
  table.header([a]),
  table.header(level: 2, [b]),
  table.footer([c])
)

--- grid-subheaders-alone-with-footer-no-orphan-prevention paged ---
#set page(height: 5.3em)
#table(
  table.header([L1]),
  table.header(level: 2, [L2]),
  table.footer([a])
)

--- grid-subheaders-alone-with-gutter-and-footer-no-orphan-prevention paged ---
#set page(height: 5.5em)
#table(
  gutter: 4pt,
  table.header([L1]),
  table.header(level: 2, [L2]),
  table.footer([a])
)

--- grid-subheaders-too-large-non-repeating-orphan-before-auto paged ---
#set page(height: 8em)
#grid(
  grid.header([1]),
  grid.header([a\ ] * 2, level: 2, repeat: false),
  grid.header([2], level: 3),
  [b\ b\ b],
)

--- grid-subheaders-too-large-repeating-orphan-before-auto paged ---
#set page(height: 8em)
#grid(
  grid.header([1]),
  grid.header([a\ ] * 2, level: 2, repeat: true),
  grid.header([2], level: 3),
  rect(width: 10pt, height: 3em, fill: red),
)

--- grid-subheaders-too-large-repeating-orphan-before-relative paged ---
#set page(height: 8em)
#grid(
  rows: (auto, auto, auto, 3em),
  grid.header([1]),
  grid.header([a\ ] * 2, level: 2, repeat: true),
  grid.header([2], level: 3),
  rect(width: 10pt, height: 3em, fill: red),
)

--- grid-subheaders-too-large-non-repeating-orphan-before-relative paged ---
#set page(height: 8em)
#grid(
  rows: (auto, auto, auto, 3em),
  grid.header([1]),
  grid.header([a\ ] * 2, level: 2, repeat: false),
  grid.header([2], level: 3),
  rect(width: 10pt, height: 3em, fill: red),
)
//@@ file tests/suite/layout_hide.typ
// Test the `hide` function.

--- hide-text paged ---
AB #h(1fr) CD \
#hide[A]B #h(1fr) C#hide[D]

--- hide-line paged ---
Hidden:
#hide[#line(length: 100%)]
#line(length: 100%)

--- hide-table paged ---
Hidden:
#hide(table(rows: 2, columns: 2)[a][b][c][d])
#table(rows: 2, columns: 2)[a][b][c][d]

--- hide-polygon paged ---
Hidden:
#hide[
  #polygon((20%, 0pt),
    (60%, 0pt),
    (80%, 2cm),
    (0%,  2cm),)
]
#polygon((20%, 0pt),
  (60%, 0pt),
  (80%, 2cm),
  (0%,  2cm),)

--- hide-rect paged ---
#set rect(
  inset: 8pt,
  fill: rgb("e4e5ea"),
  width: 100%,
)

Hidden:
#hide[
#grid(
  columns: (1fr, 1fr, 2fr),
  rows: (auto, 40pt),
  gutter: 3pt,
  rect[A],
  rect[B],
  rect[C],
  rect(height: 100%)[D],
)
]
#grid(
  columns: (1fr, 1fr, 2fr),
  rows: (auto, 40pt),
  gutter: 3pt,
  rect[A],
  rect[B],
  rect[C],
  rect(height: 100%)[D],
)

--- hide-list paged ---
Hidden:
#hide[
- 1
- 2
  1. A
  2. B
- 3
]


- 1
- 2
  1. A
  2. B
- 3

--- hide-image paged ---
Hidden:
#hide(image("/assets/images/tiger.jpg", width: 5cm, height: 1cm,))

#image("/assets/images/tiger.jpg", width: 5cm, height: 1cm,)

--- issue-622-hide-meta-cite paged ---
// Test that metadata of hidden stuff stays available.
#set cite(style: "chicago-shortened-notes")

A pirate. @arrgh \
#set text(2pt)
#hide[
  A @arrgh pirate.
  #bibliography("/assets/bib/works.bib")
]

--- issue-622-hide-meta-outline paged ---
#set text(8pt)
#outline()
#set text(2pt)
#hide(block(grid(
  [= A],
  [= B],
  block(grid(
    [= C],
    [= D],
  ))
)))
//@@ file tests/suite/layout_inline_baseline.typ
// Test baseline handling.

--- baseline-text paged ---
Hi #text(1.5em)[You], #text(0.75em)[how are you?]

Our cockatoo was one of the
#text(baseline: -0.2em)[#box(circle(radius: 2pt)) first]
#text(baseline: 0.2em)[birds #box(circle(radius: 2pt))]
that ever learned to mimic a human voice.

--- baseline-box paged ---
Hey #box(baseline: 40%, image("/assets/images/tiger.jpg", width: 1.5cm)) there!

Nice to #box(inset: 1em, stroke: red)[meet] you.

How #box(baseline: (at: auto, shift: -1em))[are] you?

Doing #box(baseline: (at: auto, shift: 1em))[just\ fine]!

--- baseline-box-align paged ---
#bounds[Hey #box(baseline: auto, rect(width: 1.5cm, height: 3em, fill: green)) there!]
#bounds[Hey #box(baseline: top, rect(width: 1.5cm, height: 3em, fill: green)) there!]
#bounds[Hey #box(baseline: horizon, rect(width: 1.5cm, height: 3em, fill: green)) there!]
#bounds[Hey #box(baseline: bottom, rect(width: 1.5cm, height: 3em, fill: green)) there!]
#bounds[Hey #box(baseline: (at: horizon, shift: -5pt), rect(width: 1.5cm, height: 3em, fill: green)) there!]
#bounds[Hey #box(baseline: (at: bottom, shift: 50%), rect(width: 1.5cm, height: 3em, fill: green)) there!]
#bounds[Hey #box(baseline: (at: auto, shift: -0.5em), rect(width: 1.5cm, height: 3em, fill: green)) there!]

--- baseline-inline-math paged ---
#bounds[$sum_(i = 1)^n i/pi = 10$]

It is $a_j = sum_(i in A_j) q_i$ of value $(p_i)/(q_i)$ and $abs({X_(i_j)})$.

#show math.equation: box
It is $a_j = sum_(i in A_j) q_i$ of value $(p_i)/(q_i)$ and $abs({X_(i_j)})$.

--- baseline-display-math paged ---
#bounds[$ sum_(i = 1)^n i/pi = 10 $]
#bounds[$ (x + y + z)/(2 dot.c 3) $]
#bounds({
  set math.equation(numbering: "(1)")
  $ sum_(i = 1)^n i/pi = 10 $
})

--- baseline-breakable-display-math paged ---
#set page(height: 5em)
#show math.equation: set block(breakable: true)

#bounds[$
  sum_(i = 1)^n i/pi &= 1 \
  sum_(i = 1)^n i/pi &= 2 \
  sum_(i = 1)^n i/pi &= 3
$]

--- baseline-paragraph paged ---
#bounds[
  Hello world!

  Bye!
]
#bounds[#text(2em)[world!] Hello!

  Cya!
]

--- baseline-isolated-box paged ---
#bounds(box(baseline: 5em)[Hello!])

#bounds[Hello! #box(baseline: 5em)[Hello!] Hello!]

--- baseline-move paged ---
#bounds(move(dy: 1em)[Test])
#bounds[Hello #box(move(dy: 1em)[Test])]
#bounds[Hello #box(move(dy: 1em)[Test]) #box(move(dy: 2em)[Testt])]

--- baseline-scale paged ---
#bounds([abc] + box(scale(50%)[World!]))
#bounds([abc] + box(scale(x: 50%)[World!]))
#bounds([abc] + box(scale(y: 50%)[World!]))
#bounds([abc] + box(scale(y: -100%)[World!]))

#bounds([abc] + box(scale(50%, reflow: true)[World!]))
#bounds([abc] + box(scale(x: 50%, reflow: true)[World!]))
#bounds([abc] + box(scale(y: 50%, reflow: true)[World!]))
#bounds([abc] + box(scale(y: -100%, reflow: true)[World!]))

--- baseline-scale-multiline paged ---
#bounds([abc] + box(scale(50%)[Hi\ World!]))
#bounds([abc] + box(scale(y: -100%)[Hi\ World!]))
#bounds([abc] + box(scale(50%, reflow: true)[Hi\ World!]))
#bounds([abc] + box(scale(y: -100%, reflow: true)[Hi\ World!]))

--- baseline-rotate paged ---
#bounds([abc] + box(rotate(30deg, origin: top)[World!]))
#bounds([abc] + box(rotate(30deg)[World!]))
#bounds([abc] + box(rotate(180deg)[World!]))
#bounds([abc] + box(rotate(30deg, origin: bottom)[World!]))
#bounds([abc] + box(rotate(30deg, reflow: true, origin: top)[World!]))
#bounds([abc] + box(rotate(30deg, reflow: true)[World!]))
#bounds([abc] + box(rotate(180deg, reflow: true)[World!]))
#bounds([abc] + box(rotate(30deg, reflow: true, origin: bottom)[World!]))

--- baseline-rotate-multiline paged ---
#bounds([abc] + box(rotate(30deg)[Hi\ World!]))
#bounds([abc] + box(rotate(180deg)[Hi\ World!]))
#bounds([abc] + box(rotate(30deg, reflow: true)[Hi\ World!]))
#bounds([abc] + box(rotate(180deg, reflow: true)[Hi\ World!]))

--- baseline-skew paged ---
#bounds([abc] + box(skew(ax: 30deg)[World!]))
#bounds([abc] + box(skew(ay: 30deg)[World!]))
#bounds([abc] + box(skew(ax: 200deg, ay: 200deg)[World!]))

#bounds([abc] + box(skew(ax: 30deg, reflow: true)[World!]))
#bounds([abc] + box(skew(ay: 30deg, reflow: true)[World!]))
#bounds([abc] + box(skew(ax: 200deg, ay: 200deg, reflow: true)[World!]))

--- baseline-skew-multiline paged ---
#bounds([abc] + box(skew(ax: 30deg)[Hi\ World!]))
#bounds([abc] + box(skew(ay: 30deg)[Hi\ World!]))

#bounds([abc] + box(skew(ax: 30deg, reflow: true)[Hi\ World!]))
#bounds([abc] + box(skew(ay: 30deg, reflow: true)[Hi\ World!]))

--- baseline-shapes paged ---
#bounds[abc #box(circle[Hey]) #box(circle())]
#bounds[abc #box(square[hey!]) #box(square())]
#bounds[abc #box(ellipse[hey!]) #box(ellipse(width: 2em, height: 1em))]

--- issue-2214-baseline-math paged ---
// The math content should also be affected by the TextElem baseline.
hello #text(baseline: -5pt)[123 #sym.WW\orld]\
hello #text(baseline: -5pt)[$123 WW#text[or]$ld]\
//@@ file tests/suite/layout_inline_bidi.typ
// Test bidirectional text and language configuration.

--- bidi-en-he-top-level paged ---
// Test reordering with different top-level paragraph directions.
#let content = par[Text טֶקסט]
#text(lang: "he", content)
#text(lang: "de", content)

--- bidi-consecutive-embedded-ltr-runs paged ---
// Test that consecutive, embedded LTR runs stay LTR.
// Here, we have two runs: "A" and italic "B".
#let content = par[أنت A#emph[B]مطرC]
#set text(font: ("PT Sans", "Noto Sans Arabic"))
#text(lang: "ar", content)
#text(lang: "de", content)

--- bidi-consecutive-embedded-rtl-runs paged ---
// Test that consecutive, embedded RTL runs stay RTL.
// Here, we have three runs: "גֶ", bold "שֶׁ", and "ם".
#let content = par[Aגֶ#strong[שֶׁ]םB]
#set text(font: ("Libertinus Serif", "Noto Serif Hebrew"))
#text(lang: "he", content)
#text(lang: "de", content)

--- bidi-nesting paged ---
// Test embedding up to level 4 with isolates.
#set text(dir: rtl)
א\u{2066}A\u{2067}Bב\u{2069}?

--- bidi-manual-linebreak paged ---
// Test hard line break (leads to two paragraphs in unicode-bidi).
#set text(lang: "ar", font: ("Noto Sans Arabic", "PT Sans"))
Life المطر هو الحياة \
الحياة تمطر is rain.

--- bidi-spacing paged ---
// Test spacing.
L #h(1cm) ריווחR \
Lריווח #h(1cm) R

--- bidi-obj paged ---
// Test inline object.
#set text(lang: "he")
קרנפיםRh#box(image("/assets/images/rhino.png", height: 11pt))inoחיים

--- bidi-whitespace-reset paged ---
// Test whether L1 whitespace resetting destroys stuff.
#set text(font: ("Libertinus Serif", "Noto Sans Arabic"))
الغالب #h(70pt) ن#" "ة

--- bidi-explicit-dir paged ---
// Test explicit dir
#set text(dir: rtl)
#text("8:00 - 9:00", dir: ltr) בבוקר
#linebreak()
ב #text("12:00 - 13:00", dir: ltr) בצהריים

--- bidi-raw paged ---
// Mixing raw
#set text(lang: "he")
לדוג. `if a == b:` זה תנאי
#set raw(lang: "python")
לדוג. `if a == b:` זה תנאי

#show raw: set text(dir:rtl)
לתכנת בעברית `אם א == ב:`

--- bidi-vertical eval ---
// Test setting a vertical direction.
// Error: 16-19 text direction must be horizontal
#set text(dir: ttb)

--- issue-1373-bidi-tofus paged ---
// Test that shaping missing characters in both left-to-right and
// right-to-left directions does not cause a crash.
#"\u{590}\u{591}\u{592}\u{593}"

#"\u{30000}\u{30001}\u{30002}\u{30003}"

--- issue-5490-bidi-invalid-range paged ---
#set text(lang: "he")
#set raw(lang: "python")
#set page(width: 240pt)
בדיקה האם מספר מתחלק במספר אחר. לדוגמה `if a % 2 == 0`

--- issue-5490-bidi-invalid-range-2 paged ---
#table(
  columns: (1fr, 1fr),
  lines(6),
  [
    #text(lang: "ar", font: ("Libertinus Serif", "Noto Sans Arabic"))[مجرد نص مؤقت لأغراض العرض التوضيحي. ]
    #text(lang: "ar")[سلام]
  ],
)

--- issue-5276-shaping-consecutive-ltr-with-lang paged ---
#let a = text(lang: "ar")[\u{645}]
#a#a
//@@ file tests/suite/layout_inline_cjk.typ
// Test CJK-specific features.

--- text-chinese-basic paged ---
// Test basic Chinese text from Wikipedia.
#set text(font: "Noto Serif CJK SC")

是美国广播公司电视剧《迷失》第3季的第22和23集，也是全剧的第71集和72集
由执行制作人戴蒙·林道夫和卡尔顿·库斯编剧，导演则是另一名执行制作人杰克·本德
节目于2007年5月23日在美国和加拿大首播，共计吸引了1400万美国观众收看
本集加上插播广告一共也持续有两个小时

--- text-cjk-latin-spacing paged ---
#set page(width: 50pt + 10pt, margin: (x: 5pt))
#set text(lang: "zh", font: "Noto Serif CJK SC", cjk-latin-spacing: auto)
#set par(justify: true)

中文，中12文1中，文12中文

中文，中ab文a中，文ab中文

#set text(cjk-latin-spacing: none)

中文，中12文1中，文12中文

中文，中ab文a中，文ab中文

--- cjk-punctuation-adjustment-1 paged ---
#set page(width: 15em)

// In the following example, the space between 》！ and ？ should be squeezed.
// because zh-CN follows GB style
#set text(lang: "zh", region: "CN", font: "Noto Serif CJK SC")
原来，你也玩《原神》！？

// However, in the following example, the space between 》！ and ？ should not be squeezed.
// because zh-TW does not follow GB style
#set text(lang: "zh", region: "TW", font: "Noto Serif CJK TC")
原來，你也玩《原神》！ ？

#set text(lang: "zh", region: "CN", font: "Noto Serif CJK SC")
「真的吗？」

#set text(lang: "ja", font: "Noto Serif CJK JP")
「本当に？」

--- cjk-punctuation-adjustment-2 paged ---
#set text(lang: "zh", region: "CN", font: "Noto Serif CJK SC")
《书名〈章节〉》 // the space between 〉 and 》 should be squeezed

〔茸毛〕：很细的毛 // the space between 〕 and ： should be squeezed

--- cjk-punctuation-adjustment-3 paged ---
#set page(width: 21em)
#set text(lang: "zh", region: "CN", font: "Noto Serif CJK SC")

// These examples contain extensive use of Chinese punctuation marks,
// from 《Which parentheses should be used when applying parentheses?》.
// link: https://archive.md/2bb1N


（〔中〕医、〔中〕药、技）系列评审

（长三角［长江三角洲］）（GB/T 16159—2012《汉语拼音正词法基本规则》）

【爱因斯坦（Albert Einstein）】物理学家

〔（2009）民申字第1622号〕

“江南海北长相忆，浅水深山独掩扉。”（［唐］刘长卿《会赦后酬主簿所问》）

参看1378页〖象形文字〗。（《现代汉语词典》修订本）

--- issue-2538-cjk-latin-spacing-before-linebreak paged ---
// Issue #2538
#set text(cjk-latin-spacing: auto)

abc字

abc字#linebreak()

abc字#linebreak()
母

abc字\
母

--- issue-6539-cjk-latin-spacing-at-manual-linebreak paged ---
// Issue #6539
#set text(cjk-latin-spacing: auto)
#set box(width: 2.3em, stroke: (x: green))

#box(align(end)[甲国\ T国])

#box(align(end)[乙国 \ T国])

#box(align(end)[丙国 T国])

#box(align(end)[丁国T国])

--- issue-2650-cjk-latin-spacing-meta paged ---
测a试

测#context [a]试

--- issue-7113-cjk-latin-spacing-shift paged ---
孔乙己#super[1]与上大人#super[2]。

孔乙己#super[A应保留spacing]与上大人#super[B]。

时间#footnote[有空白]

时间#sub[123]#super[时间]B
//@@ file tests/suite/layout_inline_hyphenate.typ
// Test hyphenation.

--- hyphenate paged ---
// Test hyphenating english and greek.
#set text(hyphenate: true)
#set page(width: auto)
#grid(
  columns: (50pt, 50pt),
  [Warm welcomes to Typst.],
  text(lang: "el")[διαμερίσματα. \ λατρευτός],
)

--- hyphenate-off-temporarily paged ---
// Test disabling hyphenation for short passages.
#set page(width: 110pt)
#set text(hyphenate: true)

Welcome to wonderful experiences. \
Welcome to `wonderful` experiences. \
Welcome to #text(hyphenate: false)[wonderful] experiences. \
Welcome to wonde#text(hyphenate: false)[rf]ul experiences. \

// Test enabling hyphenation for short passages.
#set text(hyphenate: false)
Welcome to wonderful experiences. \
Welcome to wo#text(hyphenate: true)[nd]erful experiences. \

--- hyphenate-between-shape-runs paged ---
// Hyphenate between shape runs.
#set page(width: 80pt)
#set text(hyphenate: true)
It's a #emph[Tree]beard.

--- hyphenate-shy paged ---
// Test shy hyphens.
#set text(lang: "de", hyphenate: true)
#grid(
  columns: 2 * (20pt,),
  gutter: 20pt,
  [Barankauf],
  [Bar-?ankauf],
)

--- hyphenate-punctuation paged ---
// This sequence would confuse hypher if we passed trailing / leading
// punctuation instead of just the words. So this tests that we don't
// do that. The test passes if there's just one hyphenation between
// "net" and "works".
#set page(width: 60pt)
#set text(hyphenate: true)
#h(6pt) networks, the rest.

--- hyphenate-outside-of-words paged ---
// More tests for hyphenation of non-words.
#set text(hyphenate: true)
#block(width: 0pt, "doesn't")
#block(width: 0pt, "(OneNote)")
#block(width: 0pt, "(present)")

#set text(lang: "de")
#block(width: 0pt, "(bzw.)")

--- hyphenate-pt-repeat-hyphen-natural-word-breaking paged ---
// The word breaker naturally breaks arco-da-velha at arco-/-da-velha,
// so we shall repeat the hyphen, even that hyphenate is set to false.
#set page(width: 4cm)
#set text(lang: "pt")

Alguma coisa no arco-da-velha é algo que está muito longe.

--- hyphenate-pt-repeat-hyphen-hyphenate-true paged ---
#set page(width: 4cm)
#set text(lang: "pt", hyphenate: true)

Alguma coisa no arco-da-velha é algo que está muito longe.

--- hyphenate-pt-repeat-hyphen-hyphenate-true-with-emphasis paged ---
#set page(width: 4cm)
#set text(lang: "pt", hyphenate: true)

Alguma coisa no _arco-da-velha_ é algo que está muito longe.

--- hyphenate-pt-no-repeat-hyphen paged ---
#set page(width: 4cm)
#set text(lang: "pt", hyphenate: true)

Um médico otorrinolaringologista cuida da garganta do paciente.

--- hyphenate-pt-dash-emphasis paged ---
// If the hyphen is followed by a space we shall not repeat the hyphen
// at the next line
#set page(width: 4cm)
#set text(lang: "pt", hyphenate: true)

Quebabe é a -melhor- comida que existe.

--- hyphenate-es-repeat-hyphen paged ---
#set page(width: 6cm)
#set text(lang: "es", hyphenate: true)

Lo que entendemos por nivel léxico-semántico, en cuanto su sentido más
gramatical: es aquel que estudia el origen y forma de las palabras de
un idioma.

--- hyphenate-es-capitalized-names paged ---
// If the hyphen is followed by a capitalized word we shall not repeat
//  the hyphen at the next line
#set page(width: 6.2cm)
#set text(lang: "es", hyphenate: true)

Tras el estallido de la contienda Ruiz-Giménez fue detenido junto a sus
dos hermanos y puesto bajo custodia por las autoridades republicanas, con
el objetivo de protegerle de las patrullas de milicianos.

--- hyphenate-repeat-style paged ---
// Ensure that a repeated hard hyphen keeps its styles.
#set page(width: 2cm)
#set text(lang: "es")
Hello-#text(red)[world]

--- costs-widow-orphan paged ---
#set page(height: 60pt)

#let sample = lorem(12)

#sample
#pagebreak()
#set text(costs: (widow: 0%, orphan: 0%))
#sample

--- costs-runt-avoid paged ---
#set par(justify: true)

#let sample = [please avoid runts in this text.]

#sample
#pagebreak()
#set text(costs: (runt: 10000%))
#sample

--- costs-runt-allow paged ---
#set par(justify: true)
#set text(size: 6pt)

#let sample = [a a a a a a a a a a a a a a a a a a a a a a a a a]

#sample
#pagebreak()
#set text(costs: (runt: 0%))
#sample

--- costs-hyphenation-avoid paged ---
#set par(justify: true)

#let sample = [we've increased the hyphenation cost.]

#sample
#pagebreak()
#set text(costs: (hyphenation: 10000%))
#sample

--- costs-invalid-type eval ---
// Error: 18-37 expected ratio, found auto
#set text(costs: (hyphenation: auto))

--- costs-invalid-key eval ---
// Error: 18-52 unexpected key "invalid-key", valid keys are "hyphenation", "runt", "widow", and "orphan"
#set text(costs: (hyphenation: 1%, invalid-key: 3%))

--- costs-access paged empty ---
#set text(costs: (hyphenation: 1%, runt: 2%))
#set text(costs: (widow: 3%))
#context test(text.costs, (hyphenation: 1%, runt: 2%, widow: 3%, orphan: 100%))

--- issue-hyphenate-after-tag paged ---
// Ensure that an invisible tag does not prevent hyphenation.
#set page(width: 50pt)
#set text(hyphenate: true)
#show "Tree": emph
#show emph: set text(red)
#show emph: it => it + metadata(none)
Treebeard
//@@ file tests/suite/layout_inline_justify.typ
--- justify paged ---
#set page(width: 180pt)
#set par(justify: true, first-line-indent: 14pt, spacing: 5pt, leading: 5pt)

This text is justified, meaning that spaces are stretched so that the text
forms a "block" with flush edges at both sides.

First line indents and hyphenation play nicely with justified text.

--- justify-knuth-story paged large ---
#set page(width: auto, height: auto)
#set par(leading: 4pt, justify: true)
#set text(font: "New Computer Modern")

#let story = [
  In olden times when wishing still helped one, there lived a king whose
  daughters were all beautiful; and the youngest was so beautiful that the sun
  itself, which has seen so much, was astonished whenever it shone in her face.
  Close by the king’s castle lay a great dark forest, and under an old lime-tree
  in the forest was a well, and when the day was very warm, the king’s child
  went out into the forest and sat down by the side of the cool fountain; and
  when she was bored she took a golden ball, and threw it up on high and caught
  it; and this ball was her favorite plaything.
]

#let column(title, linebreaks, hyphenate) = {
  rect(inset: 0pt, width: 132pt, fill: rgb("eee"))[
    #set par(linebreaks: linebreaks)
    #set text(hyphenate: hyphenate)
    #strong(title) \ #story
  ]
}

#grid(
  columns: 3,
  gutter: 10pt,
  column([Simple without hyphens], "simple", false),
  column([Simple with hyphens], "simple", true),
  column([Optimized with hyphens], "optimized", true),
)

--- justify-manual-linebreak paged ---
// Test that lines with hard breaks aren't justified.
#set par(justify: true)
A B C \
D

--- justify-justified-linebreak paged ---
// Test forced justification with justified break.
A B C #linebreak(justify: true)
D E F #linebreak(justify: true)

--- justify-basically-empty paged ---
// Test that there are no hick-ups with justification enabled and
// basically empty paragraph.
#set par(justify: true)
#""

--- justify-shrink-last-line paged ---
// Test that the last line can be shrunk
#set page(width: 155pt)
#set par(justify: true)
This text can be fitted in one line.

--- justify-avoid-runts paged ---
// Test that runts are avoided when it's not too costly to do so.
#set page(width: 124pt)
#set par(justify: true)
#for i in range(0, 20) {
	"a b c "
}
#"d"

--- justify-no-leading-spaces paged ---
// Test that justification cannot lead to a leading space
#set par(justify: true)
#set text(size: 12pt)
#set page(width: 45mm, height: auto)

lorem ipsum 1234, lorem ipsum dolor sit amet

#"  leading whitespace should still be displayed"

--- justify-code-blocks paged ---
// Test that justification doesn't break code blocks
#set par(justify: true)

```cpp
int main() {
  printf("Hello world\n");
  return 0;
}
```

--- justify-limits-glyph-shrink-only paged ---
#set text(hyphenate: false, overhang: false)
#set par(
  justify: true,
  justification-limits: (
    spacing: (min: 100%, max: 100%),
    tracking: (min: -0.1em, max: 0em)
  )
)

#block(fill: aqua.lighten(50%), width: 100%, lorem(10))

--- justify-limits-glyph-grow-only paged ---
#set text(hyphenate: false, overhang: false)
#set par(
  justify: true,
  justification-limits: (
    spacing: (min: 100%, max: 100%),
    tracking: (min: 0em, max: 0.5em)
  )
)

#block(fill: aqua.lighten(50%), width: 100%, lorem(10))

--- justify-limits-tight-overstretch paged ---
#set par(justify: true)
#set text(hyphenate: false)
#let with-limits(..args, body) = {
  set par(justification-limits: args.named())
  body
}

#let it = lorem(3) + linebreak(justify: true)
#let spacer = box(width: 1fr, height: 5pt, fill: aqua)

// Default cost => Spaces are stretched, glyphs are not.
#it

// No adjustments allowed => Spaces are still stretched, but incur
// very high cost while doing so.
#with-limits(
  spacing: (min: 100%, max: 100%),
  tracking: (min: 0em, max: 0em),
  it
)

// Slight tracking is allowed => Spaces are still stretched
// because just tracking is not enough.
#with-limits(
  spacing: (min: 100%, max: 100%),
  tracking: (min: 0em, max: 0.05em),
  it
)

// A ton of tracking is allowed => No space stretching occurs
// anymore.
#with-limits(
  spacing: (min: 100%, max: 150%),
  tracking: (min: 0em, max: 5em),
  it
)

// Test folding against default.
#{
  set par(justification-limits: (tracking: (min: 0em, max: 5em)))
  it
}

// Test folding against a custom value.
#{
  set par(justification-limits: (tracking: (min: 0em, max: 5em)))
  set par(justification-limits: (spacing: (min: 100%, max: 100%)))
  it
}

--- justify-limits-tracking-wrong-type eval ---
// Error: 1:32-4:2 `min` value of `tracking` is invalid (expected length, found ratio)
#set par(justification-limits: (
  spacing: (min: 100%, max: 100%),
  tracking: (min: 90%, max: 110%),
))

--- justify-limits-tracking-min-positive eval ---
// Error: 32-65 `min` value of `tracking` is invalid (length must be negative or zero)
#set par(justification-limits: (tracking: (min: 1em, max: -1em)))

--- justify-limits-tracking-max-negative eval ---
// Error: 32-66 `max` value of `tracking` is invalid (length must be positive or zero)
#set par(justification-limits: (tracking: (min: -1em, max: -1em)))

--- justify-limits-spacing-max-negative eval ---
// Error: 32-78 `max` value of `spacing` is invalid (length must be positive or zero)
#set par(justification-limits: (spacing: (min: 100% - 10pt, max: 120% - 1pt)))

--- justify-limits-spacing-ratio-negative eval ---
// Error: 32-76 `min` value of `spacing` is invalid (ratio must be positive)
#set par(justification-limits: (spacing: (min: -50% - 1pt, max: 50% + 1pt)))

--- justify-chinese paged ---
// In Chinese typography, line length should be multiples of the character size
// and the line ends should be aligned with each other. Most Chinese
// publications do not use hanging punctuation at line end.
#set page(width: auto)
#set par(justify: true)
#set text(lang: "zh", font: "Noto Serif CJK SC")

#rect(inset: 0pt, width: 80pt, fill: rgb("eee"))[
  中文维基百科使用汉字书写，汉字是汉族或华人的共同文字，是中国大陆、新加坡、马来西亚、台湾、香港、澳门的唯一官方文字或官方文字之一。25.9%，而美国和荷兰则分別占13.7%及8.2%。近年來，中国大陆地区的维基百科编辑者正在迅速增加；
]

--- justify-japanese paged ---
// Japanese typography is more complex, make sure it is at least a bit sensible.
#set page(width: auto)
#set par(justify: true)
#set text(lang: "ja", font: ("Libertinus Serif", "Noto Serif CJK JP"))
#rect(inset: 0pt, width: 80pt, fill: rgb("eee"))[
  ウィキペディア（英: Wikipedia）は、世界中のボランティアの共同作業によって執筆及び作成されるフリーの多言語インターネット百科事典である。主に寄付に依って活動している非営利団体「ウィキメディア財団」が所有・運営している。

  専門家によるオンライン百科事典プロジェクトNupedia（ヌーペディア）を前身として、2001年1月、ラリー・サンガーとジミー・ウェールズ（英: Jimmy Donal "Jimbo" Wales）により英語でプロジェクトが開始された。
]

--- justify-whitespace-adjustment paged ---
// Test punctuation whitespace adjustment
#set page(width: auto)
#set text(lang: "zh", font: "Noto Serif CJK SC")
#set par(justify: true)
#rect(inset: 0pt, width: 80pt, fill: rgb("eee"))[
  “引号测试”，还，

  《书名》《测试》下一行

  《书名》《测试》。
]

「『引号』」。“‘引号’”。

--- justify-variants paged ---
// Test Variants of Mainland China, Hong Kong, and Japan.

// 17 characters a line.
#set page(width: 170pt + 10pt, margin: (x: 5pt))
#set text(lang: "zh", font: "Noto Serif CJK SC")
#set par(justify: true)

孔雀最早见于《山海经》中的《海内经》：“有孔雀。”东汉杨孚著《异物志》记载，岭南：“孔雀，其大如大雁而足高，毛皆有斑纹彩，捕而蓄之，拍手即舞。”

#set text(lang: "zh", region: "hk", font: "Noto Serif CJK TC")
孔雀最早见于《山海经》中的《海内经》：「有孔雀。」东汉杨孚著《异物志》记载，岭南：「孔雀，其大如大雁而足高，毛皆有斑纹彩，捕而蓄之，拍手即舞。」

--- justify-punctuation-adjustment paged ---
// Test punctuation marks adjustment in justified paragraph.

// The test case includes the following scenarios:
// - Compression of punctuation marks at line start or line end
// - Adjustment of adjacent punctuation marks

#set page(width: 110pt + 10pt, margin: (x: 5pt))
#set text(lang: "zh", font: "Noto Serif CJK SC")
#set par(justify: true)

标注在字间的标点符号（乙式括号省略号以外）通常占一个汉字宽度，使其易于识别、适合配置及排版，有些排版风格完全不对标点宽度进行任何调整。但是为了让文字体裁更加紧凑易读，，，以及执行3.1.4 行首行尾禁则时，就需要对标点符号的宽度进行调整。是否调整取决于……

--- justify-without-justifiables paged ---
// Test breaking a line without justifiables.
#set par(justify: true)
#block(width: 1cm, fill: aqua, lorem(2))

--- issue-2419-justify-hanging-indent paged ---
// Test that combination of justification and hanging indent doesn't result in
// an underfull first line.
#set par(hanging-indent: 2.5cm, justify: true)
#lorem(5)

--- issue-4651-justify-bad-bound paged ---
// Test that overflow does not lead to bad bounds in paragraph optimization.
#set par(justify: true)
#block(width: 0pt)[A B]

--- issue-5360-unnecessary-hyphenation paged ---
// Test whether `Formal` would be in one line.
#set par(justify: true)
#table(columns: 1, [Formal])

--- issue-6062-justify-cjk-latin-spacing paged ---
// Test whether cjk-latin-spacing would be stretched evenly when justified.
#set par(justify: true)
あaあ#linebreak(justify: true)
ああaa aaああ#linebreak(justify: true)

--- issue-7541-justify-with-zero-width-space paged ---
#set page(background: pad(10pt, rect(
  width: 100%,
  height: 100%,
  stroke: 0.5pt + blue
)))
#set par(justify: true)

#let space = h(100% - 3.1em)

#space;Foo Bar Buzz

#space;Foo Bar#sym.zws;Buzz

--- issue-8931-justify-trailing-tracking paged ---
// Test that trailing tracking is not retained when the line is justified, which
// would leave a gutter at the right edge.
#set page(width: 200pt)
#set text(overhang: false, tracking: 0.5em)
#set par(justify: true)
#block(width: 100%, fill: aqua, lorem(10))

--- issue-2348-justify-indented-cjk-punctuation paged ---
// Test that CJK punctuation at the start of a paragraph is adjusted even if
// the paragraph is indented.
#set page(width: auto)
#set text(lang: "ja", font: "Noto Serif CJK JP")
#set par(
  justify: true,
  first-line-indent: (amount: 1em, all: true),
  hanging-indent: 0.5em,
)

『現代仮名遣い』では、助詞の\
「へ」は「へ」と書くのがルールである。
//@@ file tests/suite/layout_inline_linebreak.typ
// Test line breaks.

--- linebreak-overflow paged ---
// Test overlong word that is not directly after a hard break.
This is a spaceexceedinglylongy.

--- linebreak-overflow-double paged ---
// Test two overlong words in a row.
Supercalifragilisticexpialidocious Expialigoricmetrioxidation.

--- linebreak-hyphen-nbsp paged ---
// Test for non-breaking space and hyphen.
There are non\u{2011}breaking~characters.

--- linebreak-narrow-nbsp paged ---
// Test for narrow non-breaking space.
#show "_": sym.space.nobreak.narrow
0.1_g, 1_g, 10_g, 100_g, 1_000_g, 10_000_g, 100_000_g, 1_000_000_g

--- linebreak-shape-run paged ---
// Test that there are no unwanted line break opportunities on run change.
This is partly emp#emph[has]ized.

--- linebreak-manual paged ---
Hard #linebreak() break.

--- linebreak-manual-directly-after-automatic paged ---
// Test hard break directly after normal break.
Hard break directly after \ normal break.

--- linebreak-manual-consecutive paged ---
// Test consecutive breaks.
Two consecutive \ \ breaks and three \ \ more.

--- linebreak-manual-trailing-multiple paged ---
// Test forcing an empty trailing line.
Trailing break \ \

--- linebreak-manual-justified paged ---
// Test justified breaks.
#set par(justify: true)
With a soft #linebreak(justify: true)
break you can force a break without #linebreak(justify: true)
breaking justification. #linebreak(justify: false)
Nice!

--- linebreak-whitespace-trimming paged ---
// Ensure that even spaces across multiple layout items are trimmed during
// line breaking.
#block(width: 15pt, box(fill: aqua, underline("A   " + text(fill: blue, " ") + "    B")))

--- issues-7168-linebreak-whitespace-trimming-justify-rtl paged ---
#set page(width: 80pt)
#set par(justify: true)
#set text(dir: rtl)

Hello From Earth

--- linebreak-thai paged ---
// Test linebreak for East Asian languages
ทีวีตรวจทานนอร์ทแฟรีเลคเชอร์โกลด์อัลบัมเชอร์รี่เย้วสโตร์กฤษณ์เคลมเยอบีร่าพ่อค้าบลูเบอร์รี่สหัสวรรษโฮปแคนูโยโย่จูนสตรอว์เบอร์รีซื่อบื้อเยนแบ็กโฮเป็นไงโดนัททอมสเตริโอแคนูวิทย์แดรี่โดนัทวิทย์แอปพริคอทเซอร์ไพรส์ไฮบริดกิฟท์อินเตอร์โซนเซอร์วิสเทียมทานโคโยตี้ม็อบเที่ยงคืนบุญคุณ

--- linebreak-cite-punctuation paged ---
// Test punctuation after citations.
#set page(width: 162pt)

They can look for the details in @netwok,
which is the authoritative source.

#bibliography("/assets/bib/works.bib")

--- linebreak-math-punctuation paged ---
// Test punctuation after math equations.
#set page(width: 85pt)

We prove $1 < 2$. \
We prove $1 < 2$! \
We prove $1 < 2$? \
We prove $1 < 2$, \
We prove $1 < 2$; \
We prove $1 < 2$: \
We prove $1 < 2$- \
We prove $1 < 2$– \
We prove $1 < 2$— \

--- linebreak-link paged ---
#link("https://example.com/(ab") \
#link("https://example.com/(ab)") \
#link("https://example.com/(paren)") \
#link("https://example.com/paren)") \
#link("https://hi.com/%%%%%%%%abcdef") \

--- linebreak-link-ip-host paged ---
// Ensure that links with IP hosts use the same URL line-breaking behavior.
#set page(width: 100pt)
#set par(justify: true)

#let link-padding = "lorem_ipsum_dolor_sit_amet"
#let links = (
  "http://127.0.0.1:8080/" + link-padding,
  "HtTpS://1example.com/" + link-padding,
  "this-is-a+cool.scheme://a.com/" + link-padding,
)

#links.map(link).join(linebreak())

// This case is not yet ideal, but it should not really happen in practice.
Ähm#link("http://127.0.0.1:8080/" + link-padding)

--- linebreak-link-justify paged ---
#set page(width: 240pt)
#set par(justify: true)

Here's a link https://url.com/data/extern12840%data_urlenc and then there are more
links #link("www.url.com/data/extern12840%data_urlenc") in my text of links
http://mydataurl/hash/12098541029831025981024980124124214/incremental/progress%linkdata_information_setup_my_link_just_never_stops_going/on?query=false

--- linebreak-link-end paged ---
// Ensure that there's no unconditional break at the end of a link.
#set page(width: 180pt, height: auto, margin: auto)
#set text(11pt)

For info see #link("https://myhost.tld").

--- issue-2105-linebreak-tofu paged ---
#linebreak()中文

--- issue-3082-chinese-punctuation paged ---
#set text(font: "Noto Serif CJK TC", lang: "zh")
#set page(width: 230pt)

課有手冬，朱得過已誰卜服見以大您即乙太邊良，因且行肉因和拉幸，念姐遠米巴急（abc0），松黃貫誰。

--- issue-80-emoji-linebreak paged ---
// Test that there are no linebreaks in composite emoji (issue #80).
#set page(width: 50pt, height: auto)
#h(99%) 🏳️‍🌈
🏳️‍🌈

--- issue-hyphenate-in-link paged ---
#set par(justify: true)

// The `linebreak()` function accidentally generated out-of-order breakpoints
// for links because it now splits on word boundaries. We avoid the link markup
// syntax because it's show rule interferes.
#"http://creativecommons.org/licenses/by-nc-sa/4.0/"

--- issue-4468-linebreak-thai paged ---
// In this bug, empty-range glyphs at line break boundaries could be duplicated.
// This happens for Thai specifically because it has both
// - line break opportunities
// - shaping that results in multiple glyphs in the same cluster
#set text(font: "Noto Sans Thai")
#h(85pt) งบิก

--- issue-5235-linebreak-optimized-without-justify paged ---
#set page(width: 207pt, margin: 15pt)
#set text(11pt)

#set par(linebreaks: "simple")
Some texts feature many longer
words. Those are often exceedingly
challenging to break in a visually
pleasing way.

#set par(linebreaks: "optimized")
Some texts feature many longer
words. Those are often exceedingly
challenging to break in a visually
pleasing way.

--- issue-5489-matrix-stray-linebreak paged ---
#table(
  columns: (70pt,) * 1,
  align: horizon + center,
  stroke: 0.6pt,
  [$mat(2241/2210,-71/1105;-71/1105,147/1105)$],
)

--- linebreak-default-ignorables paged ---
#set text(font: "Noto Sans Math")
\u{2295}\u{FE00} vs \u{2295}\u{FE00}

--- issue-1920-linebreak-guillemets paged ---
// In languages like French and German, manually added spaces before and after
// guillemets should not be breakable.
#set page(width: 125pt)

#set text(lang: "fr")
Les principales « guillemets ».\
Et les autres ‹ guillemets › en français & suisse romande.

#set text(lang: "de")
Alternative »Anführungszeichen« in DE & AT.
//@@ file tests/suite/layout_inline_overhang.typ
// Test micro-typographical shenanigans.

--- overhang paged ---
// Test hanging punctuation.
// TODO: This test was broken at some point.
#set page(width: 130pt, margin: 15pt)
#set par(justify: true, linebreaks: "simple")
#set text(size: 9pt)
#rect(inset: 0pt, fill: rgb(0, 0, 0, 0), width: 100%)[
  This is a little bit of text that builds up to
  hang-ing hyphens and dash---es and then, you know,
  some punctuation in the margin.
]

// Test hanging punctuation with RTL.
#set text(lang: "he", font: ("PT Sans", "Noto Serif Hebrew"))
בנייה נכונה של משפטים ארוכים דורשת ידע בשפה. אז בואו נדבר על מזג האוויר.

--- overhang-label paged ---
// Test that labels do not interfere with hanging punctuation.
#set page(width: 130pt)
#set par(justify: true)
#set text(size: 11pt)
#set block(stroke: 0.5pt + blue, width: 4cm)

#block[This is a text with just, the right length.]
#block[This is a text with just,<label> the right length.]

--- overhang-dash paged ---
#set page(margin: 2pt)
#set par(justify: true)
#set text(size: 11pt)
#set block(stroke: 0.5pt + blue, width: 25pt)

#block({ h(1fr); sym.wj; sym.dash.em })
#block({ h(1fr); sym.dash.em })
#set text(dir: rtl)
#block({ h(1fr); sym.dash.em })
#block({ h(1fr); sym.wj; sym.dash.em; })
#set text(dir: ltr)
#show "—": sym.wj + sym.dash.em
#block({ h(1fr); sym.dash.em })

--- issue-8130-overhang-in-show-rule paged ---
#set page(margin: 2pt)
#set par(justify: true)
#set text(size: 11pt)
#set block(stroke: 0.5pt + blue, width: 10pt)

#let f(body) = {
  show "x": "y"
  body
}

#block[#h(1fr);#f[a,] b]
#block[#h(1fr);a, b]

--- overhang-lone paged ---
// Test that lone punctuation doesn't overhang into the margin.
#set page(margin: 0pt)
#set align(end)
#set text(dir: rtl)
:
//@@ file tests/suite/layout_inline_shaping.typ
// Test shaping quirks.

--- shaping-script-separation paged ---
// Test separation by script.
#set text(font: ("Libertinus Serif", "IBM Plex Sans Devanagari"))
ABCअपार्टमेंट

// This is how it should look like.
अपार्टमेंट

// This (without the spaces) is how it would look
// if we didn't separate by script.
अ पा र् ट में ट

--- shaping-forced-script-font-feature-inhibited paged ---
// A forced `latn` script inhibits Devanagari font features.
#set text(font: ("Libertinus Serif", "IBM Plex Sans Devanagari"), script: "latn")
ABCअपार्टमेंट

--- shaping-forced-script-font-feature-enabled paged ---
// A forced `deva` script enables Devanagari font features.
#set text(font: ("Libertinus Serif", "IBM Plex Sans Devanagari"), script: "deva")
ABCअपार्टमेंट

--- issue-rtl-safe-to-break-panic paged ---
// Test that RTL safe-to-break doesn't panic even though newline
// doesn't exist in shaping output.
#set text(dir: rtl, font: "Noto Serif Hebrew")
\ ט

--- shaping-font-fallback paged ---
#set text(font: ("Libertinus Serif", "Noto Sans Arabic"))
// Font fallback for emoji.
A😀B

// Font fallback for entire text.
دع النص يمطر عليك

// Font fallback in right-to-left text.
ب🐈😀سم

// Multi-layer font fallback.
Aب😀🏞سمB

// Font fallback with composed emojis and multiple fonts.
01️⃣2

// Tofus are rendered with the first font.
A🐈ዲሞB

--- shaping-emoji-basic paged ---
// This should form a three-member family.
👩‍👩‍👦

// This should form a pride flag.
🏳️‍🌈

// Skin tone modifier should be applied.
👍🏿

// This should be a 1 in a box.
1️⃣

--- shaping-emoji-bad-zwj paged ---
// These two shouldn't be affected by a zero-width joiner.
🏞‍🌋
//@@ file tests/suite/layout_inline_text.typ
// Test OpenType features.

--- text-kerning paged ---
// Test turning kerning off.
#text(kerning: true)[Tq] \
#text(kerning: false)[Tq]

--- text-alternates-and-stylistic-sets paged ---
// Test alternates and stylistic sets.
#set text(font: "IBM Plex Serif")
a vs #text(alternates: true)[a] \
ß vs #text(stylistic-set: 5)[ß] \
10 years ago vs #text(stylistic-set: (1, 2, 3))[10 years ago]

--- text-alternates-int paged ---
// Test selecting between multiple alternates.
#set text(font: "Libertinus Serif")
#text(alternates: false, [ß]) vs #text(alternates: true, [ß]) vs #text(alternates: 2, [ß])

--- text-ligatures paged ---
// Test text turning off (standard) ligatures of the font.
#text(ligatures: false)[fi Qu] vs fi Qu \
// Test text turning on historical ligatures of the font.
abstract vs #text(historical-ligatures: true)[abstract] \
// Test text turning on discretionary ligatures of the font.
waltz vs #text(discretionary-ligatures: true)[waltz]

--- text-number-type paged ---
// Test number type.
#set text(number-type: "old-style")
0123456789 \
#text(number-type: auto)[0123456789]

--- text-number-width paged ---
// Test number width.
#text(number-width: "proportional")[0123456789] \
#text(number-width: "tabular")[3456789123] \
#text(number-width: "tabular")[0123456789]

--- text-slashed-zero-and-fractions paged ---
// Test extra number stuff.
#set text(font: "IBM Plex Serif")
0 vs. #text(slashed-zero: true)[0] \
1/2 vs. #text(fractions: true)[1/2]

--- text-features paged ---
// Test raw features array or dictionary.
#text(features: ("smcp",))[Smcp] \
fi vs. #text(features: (liga: 0))[No fi]

--- text-stylistic-set-bad-type eval ---
// Error: 26-31 expected none, integer, or array, found boolean
#set text(stylistic-set: false)

--- text-stylistic-set-out-of-bounds eval ---
// Error: 26-28 stylistic set must be between 1 and 20
#set text(stylistic-set: 25)

--- text-number-type-bad eval ---
// Error: 24-25 expected "lining", "old-style", or auto, found integer
#set text(number-type: 2)

--- text-features-bad eval ---
// Error: 21-26 expected array or dictionary, found boolean
#set text(features: false)

--- text-features-non-ascii eval ---
// Error: 21-30 tag may contain only printable ASCII characters
// Hint: 21-30 found invalid cluster `"ƒ"`
// Hint: 21-30 occurred in tag at index 0 (`"ƒeat"`)
#set text(features: ("ƒeat",))

--- text-features-bad-padding eval ---
// Error: 21-30 spaces may only appear as padding following a tag
// Hint: 21-30 occurred in tag at index 0 (`" tag"`)
#set text(features: (" tag",))

--- text-features-empty-array eval ---
// Error: 21-26 tag must be one to four characters in length
// Hint: 21-26 found 0 characters
// Hint: 21-26 occurred in tag at index 0 (`""`)
#set text(features: ("",))

--- text-features-overlong-dict eval ---
// Error: 21-41 tag must be one to four characters in length
// Hint: 21-41 found 15 characters
// Hint: 21-41 occurred in tag at index 0 (`"verylongfeature"`)
#set text(features: (verylongfeature: 0))

--- text-features-array-kv eval ---
// Error: 21-32 tag must be one to four characters in length
// Hint: 21-32 found 6 characters
// Hint: 21-32 occurred in tag at index 0 (`"feat=2"`)
// Hint: 21-32 to set features with custom values, consider supplying a dictionary
#set text(features: ("feat=2",))

--- text-features-bad-nested-type eval ---
// Error: 21-35 expected string, found boolean
// Hint: 21-35 occurred in tag at index 1 (`false`)
// Hint: 21-35 to set features with custom values, consider supplying a dictionary
#set text(features: ("tag", false))

--- text-tracking-negative paged ---
// Test tracking.
#set text(tracking: -0.01em)
I saw Zoe yӛsterday, on the tram.

--- text-tracking-changed-temporarily paged ---
// Test tracking for only part of paragraph.
I'm in#text(tracking: 0.15em + 1.5pt)[ spaace]!

--- text-tracking-mark-placement paged ---
// Test that tracking doesn't disrupt mark placement.
#set text(font: ("PT Sans", "Noto Serif Hebrew"))
#set text(tracking: 0.3em)
טֶקסט

--- text-tracking-arabic paged ---
// Test tracking in arabic text (makes no sense whatsoever)
#set text(tracking: 0.3em, font: "Noto Sans Arabic")
النص

--- text-spacing paged ---
// Test word spacing.
#set text(spacing: 1em)
My text has spaces.

--- text-spacing-relative paged ---
// Test word spacing relative to the font's space width.
#set text(spacing: 50% + 1pt)
This is tight.

--- issue-5855-misaligned-descender paged ---
foo #box[foo] foo

#set text(bottom-edge: "descender")

foo #box[foo] foo

--- issue-8372-stretched-glyphs paged ---
#set page(width: auto)
#set text(5em)
#let b(body) = {
  box(stroke: red, width: auto, height: 1.5em, {
    set align(center + horizon)
    box(stroke: blue, width: auto, height: auto, body)
  })
}
#b({
  set text(font: ("Noto Color Emoji CBDT Subset", "Libertinus Serif"), fallback: false)
  [A#emoji.checkmark.box]
})
#b({
  // Will use a COLR glyph
  set text(font: ("Noto Color Emoji", "Libertinus Serif"), fallback: false)
  [A#emoji.checkmark.box]
})

--- issue-text-tracking-cjk-punctuation-adjustment paged empty ---
// Test that tracking is not considered part of the adjustability.
#set text(lang: "zh", font: "Noto Serif CJK SC")
#let t = [。」]
#context test(
  measure(text(tracking: 1em, t)).width,
  measure(t).width + 1em.to-absolute(),
)

--- issue-text-tracking-synthesized-scripts paged empty ---
// Check that we add the right tracking amount of tracking even with scaled-down
// synthesized superscripts. Absolute tracking should stay as-is while em-based
// tracking should be relative to the script size.
#set super(typographic: false, size: 0.5em)
#context test(
  measure(super(text(tracking: 1pt + 1em)[A B])).width,
  measure(super[A B]).width + 2 * (1pt + 0.5em.to-absolute()),
)

--- issue-text-spacing-synthesized-scripts paged empty ---
#set super(typographic: false, size: 0.5em)
#context test(
  measure(super(text(spacing: 100% + 1pt + 1em)[A B])).width,
  measure(super[A B]).width + 1pt + 0.5em.to-absolute(),
)
//@@ file tests/suite/layout_layout.typ
--- layout-in-fixed-size-block paged ---
// Layout inside a block with certain dimensions should provide those dimensions.
#set page(height: 120pt)
#block(width: 60pt, height: 80pt, layout(size => [
  This block has a width of #size.width and height of #size.height
]))

--- layout-in-page-call paged ---
// Layout without any container should provide the page's dimensions, minus its margins.
#page(width: 100pt, height: 100pt, {
  layout(size => [This page has a width of #size.width and height of #size.height ])
  h(1em)
  place(left, rect(width: 80pt, stroke: blue))
})

--- issue-6822-layout-expand paged ---
// Ensure that the body of `layout` can expand and respects alignment.
#rect(width: 100%, height: 50pt, layout(_ => {
  place(top + left)[1]
  place(center + horizon)[2]
  place(bottom + right)[3]
}))

--- issue-6822-layout-multiple paged ---
// But if there are multiple children (the `layout` is not the lone child), it
// cannot expand vertically anymore, just like `pad`.
#rect(width: 100%, height: 50pt, {
  pad(y: 1pt, align(center + horizon)[1])
  layout(_ =>  place(center + horizon)[2])
})
//@@ file tests/suite/layout_length.typ
--- length-fields eval ---
// Test length fields.
#test((1pt).em, 0.0)
#test((1pt).abs, 1pt)
#test((3em).em, 3.0)
#test((3em).abs, 0pt)
#test((2em + 2pt).em, 2.0)
#test((2em + 2pt).abs, 2pt)

--- length-to-unit eval ---
// Test length unit conversions.
#let t(a, b) = assert(calc.abs(a - b) < 1e-6)

#t((500.934pt).pt(), 500.934)
#t((3.3453cm).cm(), 3.3453)
#t((4.3452mm).mm(), 4.3452)
#t((5.345in).inches(), 5.345)
#t((500.333666999pt).pt(), 500.333666999)
#t((3.523435cm).cm(), 3.523435)
#t((4.12345678mm).mm(), 4.12345678)
#t((5.333666999in).inches(), 5.333666999)
#t((4.123456789123456mm).mm(), 4.123456789123456)
#t((254cm).mm(), 2540.0)
#t((254cm).inches(), 100.0)
#t((2540mm).cm(), 254.0)
#t((2540mm).inches(), 100.0)
#t((100in).pt(), 7200.0)
#t((100in).cm(), 254.0)
#t((100in).mm(), 2540.0)
#t(5em.abs.cm(), 0.0)
#t((5em + 6in).abs.inches(), 6.0)

--- length-to-absolute paged empty ---
// Test length `to-absolute` method.
#set text(size: 12pt)
#context {
  test((6pt).to-absolute(), 6pt)
  test((6pt + 10em).to-absolute(), 126pt)
  test((10em).to-absolute(), 120pt)
}

#set text(size: 64pt)
#context {
  test((6pt).to-absolute(), 6pt)
  test((6pt + 10em).to-absolute(), 646pt)
  test((10em).to-absolute(), 640pt)
}

--- length-unit-hint eval ---
// Error: 17-19 expected length, found integer
// Hint: 17-19 a length needs a unit - did you mean 12pt?
#set text(size: 12)

--- length-ignore-em-pt-hint eval ---
// Error: 2-21 cannot convert a length with non-zero em units (`-6pt + 10.5em`) to pt
// Hint: 2-21 use `length.to-absolute()` to resolve its em component (requires context)
// Hint: 2-21 or use `length.abs.pt()` instead to ignore its em component
#(10.5em - 6pt).pt()

--- length-ignore-em-cm-hint eval ---
// Error: 2-12 cannot convert a length with non-zero em units (`3em`) to cm
// Hint: 2-12 use `length.to-absolute()` to resolve its em component (requires context)
// Hint: 2-12 or use `length.abs.cm()` instead to ignore its em component
#(3em).cm()

--- length-ignore-em-mm-hint eval ---
// Error: 2-20 cannot convert a length with non-zero em units (`-226.77pt + 93em`) to mm
// Hint: 2-20 use `length.to-absolute()` to resolve its em component (requires context)
// Hint: 2-20 or use `length.abs.mm()` instead to ignore its em component
#(93em - 80mm).mm()

--- length-ignore-em-inches-hint eval ---
// Error: 2-24 cannot convert a length with non-zero em units (`432pt + 4.5em`) to inches
// Hint: 2-24 use `length.to-absolute()` to resolve its em component (requires context)
// Hint: 2-24 or use `length.abs.inches()` instead to ignore its em component
#(4.5em + 6in).inches()

--- issue-5519-nondecimal-suffix eval ---
// Error: 2-9 binary numbers cannot have a suffix
// Hint: 2-9 try using a decimal number: `4pt`
#0b100pt

--- nondecimal-suffix-edge-cases eval ---
// Error: 2-7 octal numbers cannot have a suffix
// Hint: 2-7 try using a decimal number: `50%`
#0o62%
// Error: 2-8 hexadecimal numbers cannot have a suffix
// Hint: 2-8 try using a decimal number: `2748%`
#0xabc%
// Error: 2-11 binary numbers cannot have a suffix
// Hint: 2-11 invalid number suffix: `dag`
#0b0101dag

--- nondecimal-suffix-edge-case-delayed eval ---
// This error is delayed until the integer is evaluated.
// Error: 2-9 integer contains digits that are not valid for a hexadecimal number
// Hint: 2-9 hexadecimal numbers only allow digits 0-9, a-f, A-F
// Hint: 8-9 the digit `m` is invalid
#0xabcem

--- number-syntax-edge-cases eval ---
// Test numeric syntax edge cases with suffixes and which spans of text are
// highlighted. Valid items are those not annotated with an error comment since
// syntax is handled at parse time.

// All fine
#2em
#6.3e5em
#.5pt
#1.2E+0%
#1.2e-0%
#0.0e0deg
#0.%
// Error: 2-6 invalid number suffix: `in%`
#5in%
// Error: 2-6 invalid number suffix: `%in`
#5%in
// Error: 2-8 invalid number suffix: `hello`
#1hello
// Error: 2-7 invalid number suffix: `infr`
#1infr
// Error: 2-5 invalid floating point number: `2E`
// Hint: 2-5 invalid number suffix: `M`
#2EM
// Error: 2-8 invalid floating point number: `.1E-`
#.1E-fr
// Error: 2-16 invalid floating point number: `0.1E+`
// Hint: 2-16 invalid number suffix: `fr123e456`
#0.1E+fr123e456
// Error: 2-11 invalid floating point number: `.1e-`
// Hint: 2-11 invalid number suffix: `fr123`
#.1e-fr123.456
//@@ file tests/suite/layout_limits.typ
// Test how the layout engine reacts when reaching limits like
// zero, infinity or when dealing with NaN.

--- issue-1216-clamp-panic paged ---
#set page(height: 20pt, margin: 0pt)
#v(22pt)
#block(fill: red, width: 100%, height: 10pt, radius: 4pt)

--- issue-1918-layout-infinite-length-grid-columns paged ---
// Test that passing infinite lengths to drawing primitives does not crash Typst.
#set page(width: auto, height: auto)

// Error: 58-59 cannot expand into infinite width
#layout(size => grid(columns: (size.width, size.height))[a][b][c][d])

--- issue-1918-layout-infinite-length-grid-rows paged ---
#set page(width: auto, height: auto)

// Error: 17-66 cannot create grid with infinite height
#layout(size => grid(rows: (size.width, size.height))[a][b][c][d])

--- issue-1918-layout-infinite-length-line paged ---
#set page(width: auto, height: auto)

// Error: 17-41 cannot create line with infinite length
#layout(size => line(length: size.width))

--- issue-1918-layout-infinite-length-polygon paged ---
#set page(width: auto, height: auto)

// Error: 17-54 cannot create polygon with infinite size
#layout(size => polygon((0pt,0pt), (0pt, size.width)))
//@@ file tests/suite/layout_line-numbers.typ
--- line-numbers-enable paged ---
#set page(margin: (left: 2.5em))
#set par.line(numbering: "1")

First line \
Second line \
Third line

--- line-numbers-clearance paged ---
#set page(margin: (left: 1.5cm))
#set par.line(numbering: "1", number-clearance: 0cm)

First line \
Second line \
Third line

--- line-numbers-margin paged ---
#set page(margin: (right: 3cm))
#set par.line(numbering: "1", number-clearance: 1.5cm, number-margin: end)

First line \
Second line \
Third line

--- line-numbers-default-alignment paged ---
#set page(margin: (left: 3em))
#set par.line(numbering: "1")
a
#([\ a] * 15)

--- line-numbers-start-alignment paged ---
#set page(margin: (left: 3em))
#set par.line(numbering: "i", number-align: start)
a \
a
#pagebreak()
a \
a \
a

--- line-numbers-auto-alignment paged ---
#set page(margin: (right: 3cm))
#set par.line(numbering: "i", number-clearance: 1.5cm, number-margin: end)

First line \
Second line \
Third line

--- line-numbers-rtl paged ---
#set page(margin: (right: 3em))
#set text(dir: rtl)
#set par.line(numbering: "1")
a
#([\ a] * 15)

--- line-numbers-columns paged ---
#set page(columns: 2, margin: (x: 1.5em))
#set par.line(numbering: "1", number-clearance: 0.5em)

Hello \
Beautiful \
World
#colbreak()
Birds \
In the \
Sky

--- line-numbers-columns-alignment paged ---
#set page(columns: 2, margin: (x: 1.5em))
#set par.line(numbering: "i", number-clearance: 0.5em)

Hello \
Beautiful \
World
#colbreak()
Birds \
In the \
Sky

--- line-numbers-multi-columns paged ---
#set page(columns: 3, margin: (x: 1.5em))
#set par.line(numbering: "1", number-clearance: 0.5em)

A \
B \
C
#colbreak()
D \
E \
F
#colbreak()
G \
H \
I

--- line-numbers-columns-rtl paged ---
#set page(columns: 2, margin: (x: 1.5em))
#set par.line(numbering: "1", number-clearance: 0.5em)
#set text(dir: rtl)

Hello \
Beautiful \
World
#colbreak()
Birds \
In the \
Sky

--- line-numbers-columns-override paged ---
#set columns(gutter: 1.5em)
#set page(columns: 2, margin: (x: 1.5em))
#set par.line(numbering: "1", number-margin: end, number-clearance: 0.5em)

Hello \
Beautiful \
World
#colbreak()
Birds \
In the \
Sky

--- line-numbers-page-scope paged ---
#set page(margin: (left: 2.5em))
#set par.line(numbering: "1", numbering-scope: "page")

First line \
Second line
#pagebreak()
Back to first line \
Second line again
#page[
  Once again, first \
  And second
]
Back to first

--- line-numbers-page-scope-with-columns paged ---
#set page(margin: (x: 1.1cm), columns: 2)
#set par.line(
  numbering: "1",
  number-clearance: 0.5cm,
  numbering-scope: "page"
)

A \
A \
A
#colbreak()
B \
B \
B
#pagebreak()
One \
Two \
Three
#colbreak()
Four \
Five \
Six
#page[
  Page \
  Elem
  #colbreak()
  Number \
  Reset
]
We're back
#colbreak()
Bye!

--- line-numbers-page-scope-quasi-empty-first-column paged ---
// Ensure this case (handled separately internally) is properly handled.
#set page(margin: (x: 1.1cm), height: 2cm, columns: 2)
#set par.line(
  numbering: "1",
  number-clearance: 0.5cm,
  numbering-scope: "page"
)

First line
#colbreak()
Second line
#pagebreak()
#place[]
#box(height: 2cm)[First!]

--- line-numbers-nested-content paged ---
#set page(margin: (left: 1.5cm))
#set par.line(numbering: "1", number-clearance: 0.5cm)

#grid(
  columns: (1fr, 1fr),
  column-gutter: 0.5cm,
  inset: 5pt,
  block[A\ #box(lorem(5))], [Roses\ are\ red],
  [AAA], [],
  [], block[BBB\ CCC],
)

--- line-numbers-place-out-of-order paged ---
#set page(margin: (left: 1.5cm))
#set par.line(numbering: "1", number-clearance: 0.5cm)

#place(bottom)[Line 4]

Line 1\
Line 2\
Line 3
#v(1cm)

--- line-numbers-deduplication paged ---
#set page(margin: (left: 1.5cm))
#set par.line(numbering: "1", number-clearance: 0.5cm)

#grid(
  columns: (1fr, 1fr),
  column-gutter: 0.5cm,
  row-gutter: 5pt,
  lorem(5), [A\ B\ C],
  [DDD], [DDD],
  [This is], move(dy: 2pt)[tough]
)

--- line-numbers-deduplication-tall-line paged ---
#set page(margin: (left: 1.5cm))
#set par.line(numbering: "1", number-clearance: 0.5cm)

#grid(
  columns: (1fr, 1fr),
  column-gutter: 0.5cm,
  stroke: 0.5pt,

  grid.cell(rowspan: 2)[very #box(fill: red, height: 4cm)[tall]],
  grid.cell(inset: (y: 0.5pt))[Line 1\ Line 2\ Line 3],
  grid.cell(inset: (y: 0.5pt))[Line 4\ Line 5\ Line 6\ Line 7\ Line 8\ Line 9\ End]
)

--- line-numbers-deduplication-zero-height-number paged ---
#set page(margin: (left: 1.5cm))
#set par.line(numbering: n => move(dy: -0.6em, box(height: 0pt)[#n]), number-clearance: 0.5cm)

#grid(
  columns: (1fr, 1fr),
  column-gutter: 0.5cm,
  row-gutter: 5pt,
  lorem(5), [A\ B\ C],
  [DDD], [DDD],
  [This is], move(dy: 3pt)[tough]
)

--- line-numbers-equation-number paged ---
#set page(margin: (left: 2.5em))
#set par.line(numbering: "1")
#set math.equation(numbering: "(1)")

A
$ x $
B

--- line-numbers-pdf-2-0 pdf ---
#set pdf(standard: "2.0")
#set page(margin: (left: 2.5em))
#set par.line(numbering: "1")

First line \
Second line
//@@ file tests/suite/layout_measure.typ
--- measure paged empty ---
// Test `measure`.
#let f(lo, hi) = context {
  let h = measure[Hello].height
  assert(h > lo)
  assert(h < hi)
}
#text(10pt, f(6pt, 8pt))
#text(20pt, f(13pt, 14pt))

--- measure-baseline-none paged empty ---
// Test that `measure` exposes the baseline of content.
#context {
  let rect = rect(width: 10pt, height: 20pt)
  let baseline-less = measure(rect)
  test(baseline-less.baseline, baseline-less.height)
}

--- measure-baseline-box paged empty ---
#context {
  let text = measure[Hello]
  let boxed-text = measure(box[Hello])
  assert(text.baseline > 0pt)
  test(boxed-text.baseline, text.baseline)
}

--- measure-baseline-box-shifted paged empty ---
#context {
  let shifted = measure(box(
    width: 10pt,
    height: 20pt,
    baseline: (at: bottom, shift: 5pt),
  ))
  test(shifted.baseline, 15pt)
}

--- measure-baseline-math paged empty ---
#context {
  let math = measure($sum_(i=1)^n i$)
  let boxed-math = measure(box($sum_(i=1)^n i$))
  assert(math.baseline > 0pt)
  assert(math.baseline < math.height)
  test(boxed-math.baseline, math.baseline)
}

--- measure-given-area paged empty ---
// Test `measure` given an area.
#let text = lorem(100)

#context {
  let d1 = measure(text)
  assert(d1.width > 2000pt)
  assert(d1.height < 10pt)
  let d2 = measure(width: 400pt, height: auto, text)
  assert(d2.width < 400pt)
  assert(d2.height > 50pt)
}

--- measure-counter-width paged ---
// Measure a counter. Tests that the introspector-assisted location assignment
// is able to take `here()` from the context into account to find the closest
// matching element instead of any single one. Crucially, we need to reuse
// the same `context c.display()` to get the same span, hence `it`.
#let f(it) = context [
  Is #measure(it).width wide: #it \
]

#let c = counter("c")
#let it = context c.display()

#c.update(10000)
#f(it)
#c.update(100)
#f(it)
#c.update(1)
#f(it)

--- measure-citation-in-flow paged ---
// Try measuring a citation that appears inline with other stuff. The
// introspection-assisted location assignment will ensure that the citation
// in the measurement is matched up with the real one.
#context {
  let it = [@netwok]
  let size = measure(it)
  place(line(length: size.width))
  v(1mm)
  it + [ is cited]
}

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- measure-citation-in-flow-different-span paged ---
// When the citation has a different span, it stops working.
#context {
  // Error: 22-29 citation could not be located
  // Hint: 22-29 this citation is not stably present in the document
  // Hint: 22-29 this can be caused by measurement or introspection
  let size = measure[@netwok]
  place(line(length: size.width))
  v(1mm)
  [@netwok is cited]
}

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- measure-citation-deeply-nested paged ---
// Nested the citation deeply to test that introspector-assisted measurement
// is able to deal with memoization boundaries.
#context {
  let it = box(pad(x: 5pt, grid(stack[@netwok])))
  [#measure(it).width]
  it
}

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- measure-counter paged empty ---
// When the thing we measure appears multiple times, we measure as if it was
// the first one.
#context {
  let c = counter("c")
  let it = context c.get().first() * h(1pt)
  c.update(42) + it
  metadata(measure(it).width)
}
#context test(query(metadata).first().value, 42pt)

--- measure-counter-bundle bundle ---
// Ensure that introspector-assisted location assignment works in bundle export.
// This tests how document introspectors are combined into a bundle
// introspector.
#document("main.pdf", context {
  let c = counter("c")
  let it = context c.get().first() * h(1pt)
  c.update(42) + it
  metadata(measure(it).width)
})
#context test(query(metadata).first().value, 42pt)

--- measure-counter-multiple-times paged empty ---
// When the thing we measure appears multiple times, we measure as if it was
// the first one.
#context {
  let c = counter("c")
  let it = context c.get().first() * h(1pt)
  let u(n) = c.update(n)
  grid(columns: 5, u(17), it, u(1), it, u(5))
  metadata(measure(it).width)
}
#context test(query(metadata).first().value, 17pt)

--- issue-5180-measure-inline-math-bounds paged empty ---
#context {
  let height = measure(text(top-edge: "bounds", $x$)).height
  assert(height > 4pt)
  assert(height < 5pt)
}

--- measure-html html empty ---
#context {
  let (width, height) = measure(image("/assets/images/monkey.svg"))
  test(width, 36pt)
  test(height, 36pt)

  let measured = measure(image("/assets/images/monkey.svg"))
  test(measured.baseline, height)
}
//@@ file tests/suite/layout_pad.typ
// Test the `pad` function.

--- pad-basic paged ---
// Use for indentation.
#pad(left: 10pt, [Indented!])

// All sides together.
#set rect(inset: 0pt)
#rect(fill: conifer,
  pad(10pt, right: 20pt,
    rect(width: 20pt, height: 20pt, fill: rgb("eb5278"))
  )
)

Hi #box(pad(left: 10pt)[A]) there

--- pad-expanding-contents paged ---
// Pad can grow.
#pad(left: 10pt, right: 10pt)[PL #h(1fr) PR]

--- pad-followed-by-content paged ---
// Test that the pad element doesn't consume the whole region.
#set page(height: 6cm)
#align(left)[Before]
#pad(10pt, image("/assets/images/tiger.jpg"))
#align(right)[After]

--- pad-adding-to-100-percent paged empty ---
// Test that padding adding up to 100% does not panic.
#pad(50%)[]

--- issue-5044-pad-100-percent paged ---
#set page(width: 30pt, height: 30pt)
#pad(100%, block(width: 1cm, height: 1cm, fill: red))

--- issue-5160-unbreakable-pad paged ---
#set block(breakable: false)
#block(width: 100%, pad(x: 20pt, align(right)[A]))
//@@ file tests/suite/layout_page.typ
// Test the page class.

--- page-call-empty paged empty ---
// Just empty page.
// Should result in auto-sized page, just like nothing.
#page[]

--- page-call-styled-empty paged ---
// Just empty page with styles.
// Should result in one conifer-colored A11 page.
#page("a11", flipped: true, fill: conifer)[]

--- page-call-followed-by-pagebreak paged ---
// Just page followed by pagebreak.
// Should result in one forest-colored A11 page and one auto-sized page.
#page("a11", flipped: true, fill: forest)[]
#pagebreak()

--- page-set-forces-break paged ---
// Set width and height.
// Should result in one high and one wide page.
#set page(width: 80pt, height: 80pt)
#[#set page(width: 40pt);High]
#[#set page(height: 40pt);Wide]

// Flipped predefined paper.
#[#set page(paper: "a11", flipped: true);Flipped A11]

--- page-set-in-container paged ---
#box[
  // Error: 4-18 page configuration is not allowed inside of containers
  #set page("a4")
]

--- page-set-empty paged ---
// Empty with styles
// Should result in one conifer-colored A11 page.
#set page("a11", flipped: true, fill: conifer)

--- page-set-only-pagebreak paged ---
// Empty with styles and then pagebreak
// Should result in two forest-colored pages.
#set page(fill: forest)
#pagebreak()

--- page-set-override-thrice paged ---
// Empty with multiple page styles.
// Should result in a small white page.
#set page("a4")
#set page("a5")
#set page(width: 1cm, height: 1cm)

--- page-set-override-and-mix paged ---
// Empty with multiple page styles.
// Should result in one eastern-colored A11 page.
#set page("a4")
#set page("a5")
#set page("a11", flipped: true, fill: eastern)
#set text(font: "Roboto", white)
#smallcaps[Typst]

--- page-large paged ---
#set page("a4")

--- page-fill paged ---
// Test page fill.
#set page(width: 80pt, height: 40pt, fill: eastern)
#text(15pt, font: "Roboto", fill: white, smallcaps[Typst])
#page(width: 40pt, fill: auto, margin: (top: 10pt, rest: auto))[Hi]

--- page-fill-none paged ---
// Test disabling page fill.
// The PNG is filled with black anyway due to the test runner.
#set page(fill: none)
#rect(fill: green)

--- page-margin-uniform paged ---
// Set all margins at once.
#[
  #set page(height: 20pt, margin: 5pt)
  #place(top + left)[TL]
  #place(bottom + right)[BR]
]

--- page-margin-individual paged ---
// Set individual margins.
#set page(height: 40pt)
#[#set page(margin: (left: 0pt)); #align(left)[Left]]
#[#set page(margin: (right: 0pt)); #align(right)[Right]]
#[#set page(margin: (top: 0pt)); #align(top)[Top]]
#[#set page(margin: (bottom: 0pt)); #align(bottom)[Bottom]]

// Ensure that specific margins override general margins.
#[#set page(margin: (rest: 0pt, left: 20pt)); Overridden]

--- page-margin-inside-outside-override paged ---
#set page(height: 100pt, margin: (inside: 30pt, outside: 20pt))
#set par(justify: true)
#set text(size: 8pt)

#page(margin: (x: 20pt), {
  set align(center + horizon)
  text(20pt, strong[Title])
  v(2em, weak: true)
  text(15pt)[Author]
})

= Introduction
#lorem(35)

--- page-margin-inside paged ---
#set page(margin: (inside: 30pt))
#rect(width: 100%)[Bound]
#pagebreak()
#rect(width: 100%)[Left]

--- page-margin-inside-with-binding paged ---
// Test setting the binding explicitly.
#set page(binding: right, margin: (inside: 30pt))
#rect(width: 100%)[Bound]
#pagebreak()
#rect(width: 100%)[Right]

--- page-margin-binding-from-text-lang paged ---
// Test setting the binding implicitly.
#set page(margin: (inside: 30pt))
#set text(lang: "he")
#rect(width: 100%)[Bound]
#pagebreak()
#rect(width: 100%)[Right]

--- page-margin-left-and-outside eval ---
// Error: 19-44 `inside` and `outside` are mutually exclusive with `left` and `right`
#set page(margin: (left: 1cm, outside: 2cm))

--- page-margin-binding-bad eval ---
// Error: 20-23 must be `left` or `right`
#set page(binding: top)

--- page-marginals paged ---
#set page(
  paper: "a8",
  margin: (x: 15pt, y: 30pt),
  header: {
    text(eastern)[*Typst*]
    h(1fr)
    text(0.8em)[_Chapter 1_]
  },
  footer: context align(center)[\~ #counter(page).display() \~],
  background: context if counter(page).get().first() <= 2 {
    place(center + horizon, circle(radius: 1cm, fill: luma(90%)))
  }
)

#align(center, lines(20))

#set page(header: none, height: auto, margin: (top: 15pt, bottom: 25pt))
Z

--- page-number-align-top-right paged ---
#set page(
  height: 100pt,
  margin: 30pt,
  numbering: "(1)",
  number-align: top + right,
)

#block(width: 100%, height: 100%, fill: aqua.lighten(50%))

--- page-number-align-bottom-left paged ---
#set page(
  height: 100pt,
  margin: 30pt,
  numbering: "[1]",
  number-align: bottom + left,
)

#block(width: 100%, height: 100%, fill: aqua.lighten(50%))

--- page-number-align-left-horizon eval ---
// Error: 25-39 expected `top` or `bottom`, found horizon
#set page(number-align: left + horizon)

--- page-numbering-pdf-label paged ---
#set page(margin: (bottom: 20pt, rest: 10pt))
#let filler = lines(7)

// (i) - (ii). No style opt. because of suffix.
#set page(numbering: "(i)")
#filler
#pagebreak()
#filler

// 3 - 4. Style opt. Page Label should use /D style.
#set page(numbering: "1")
#filler
#pagebreak()
#filler

// I - IV. Style opt. Page Label should use /R style and start at 1 again.
#set page(numbering: "I / I")
#counter(page).update(1)
#filler
#pagebreak()
#filler
#pagebreak()
#filler
#pagebreak()
#filler

// Pre: ほ, Pre: ろ, Pre: は, Pre: に. No style opt. Uses prefix field entirely.
// Counter update without numbering change.
#set page(numbering: "Pre: い")
#filler
#pagebreak()
#filler
#counter(page).update(2)
#filler
#pagebreak()
#filler
#pagebreak()
#filler

// aa & ba. Style opt only for values <= 26. Page Label uses lower alphabet style.
// Repeats letter each 26 pages or uses numbering directly as prefix.
#set page(numbering: "a")
#counter(page).update(27)
#filler
#pagebreak()
#counter(page).update(53)
#filler

--- page-numbering-hint paged ---
= Heading <intro>

// Error: 21-47 cannot reference without page numbering
// Hint: 21-47 you can enable page numbering with `#set page(numbering: "1")`
Can not be used as #ref(<intro>, form: "page")

--- page-suppress-headers-and-footers paged ---
#set page(header: none, footer: none, numbering: "1")
Look, ma, no page numbers!

#pagebreak()

#set page(header: auto, footer: auto)
Default page numbers now.

--- page-numbering-huge paged ---
#set page(margin: (bottom: 20pt, rest: 0pt))
#let filler = lines(1)

// Test values greater than 32-bits
#set page(numbering: "1/1")
#counter(page).update(100000000001)
#pagebreak()
#pagebreak()

--- page-marginal-style-text-set paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
#set text(red)
Red

--- page-marginal-style-text-set-first paged ---
#set text(red)
#set page(numbering: "1", margin: (bottom: 20pt))
Red

--- page-marginal-style-text-call paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
#text(red)[Red]

--- page-marginal-style-text-call-code paged ---
#{
  set page(numbering: "1", margin: (bottom: 20pt))
  text(red)[Red]
}

--- page-marginal-style-text-call-around-page-call paged ---
#text(red, page(numbering: "1", margin: (bottom: 20pt))[Hello])

--- page-marginal-style-text-call-around-set-page paged ---
#text(red, {
  set page(numbering: "1", margin: (bottom: 20pt))
  text(style: "italic")[Hello]
})

--- page-marginal-style-text-call-around-pagebreak paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
A
#text(red)[
  #pagebreak(weak: true)
  B
]

--- page-marginal-style-show-rule paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
= Introduction

--- page-marginal-style-show-rule-with-set-page paged ---
#show heading: it => {
  set page(numbering: "1", margin: (bottom: 20pt))
  it
}

= Introduction

--- page-marginal-style-show-rule-with-page-call paged ---
#show heading: page.with(fill: aqua)

A
= Introduction
B

--- page-marginal-style-show-rule-with-pagebreak paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
#show heading: it => {
  pagebreak(weak: true)
  it
}

= Introduction

--- page-marginal-style-context paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
#show: it => context {
  set text(red)
  it
}
Hi

--- page-marginal-style-shared-initial-interaction paged ---
#set page(numbering: "1", margin: (bottom: 20pt))
A
#{
  set text(fill: red)
  pagebreak()
}
#text(fill: blue)[B]

--- page-marginal-style-empty paged ---
#set text(red)
#set page(numbering: "1", margin: (bottom: 20pt))

--- page-marginal-style-page-call paged ---
#page(numbering: "1", margin: (bottom: 20pt))[
  #set text(red)
  A
]

--- page-show-warning paged empty ---
// Warning: 2-17 `show page` is not supported and has no effect
// Hint: 2-17 customize pages with `set page(..)` instead
#show page: none

--- page-set-html html ---
// Warning: 2-16 page set rule was ignored during HTML export
#set page("a4")
Hi

--- issue-2631-page-header-ordering paged ---
#set text(6pt)
#show heading: set text(6pt, weight: "regular")
#set page(
  margin: (x: 10pt, top: 20pt, bottom: 10pt),
  height: 50pt,
  header: context {
    let prev = query(selector(heading).before(here()))
    let next = query(selector(heading).after(here()))
    let prev = if prev != () { prev.last().body }
    let next = if next != () { next.first().body }
    (prev: prev, next: next)
  }
)

= First
Hi
#pagebreak()
= Second

--- issue-4340-set-document-and-page paged ---
// Test custom page fields being applied on the last page
// if the document has custom fields.
#set document(author: "")
#set page(fill: gray)
text
#pagebreak()

--- issue-2326-context-set-page paged ---
#context [
  #set page(fill: aqua)
  On page #here().page()
]

--- issue-3671-get-from-page-call paged empty ---
#set page(margin: 5pt)
#context test(page.margin, 5pt)
#page(margin: 10pt, context test(page.margin, 10pt))

--- issue-4363-set-page-after-tag paged ---
#set page(fill: aqua)
1
#pagebreak()
#metadata(none)
#set page(fill: red)
2

--- issue-7292-page-width-auto-margin-zero paged ---
#set page(width: auto, height: 100pt, margin: 0pt)

--- issue-7292-page-height-auto-margin-zero paged ---
#set page(width: 100pt, height: auto, margin: 0pt)

--- issue-7292-page-size-auto-margin-zero paged ---
#set page(width: auto, height: auto, margin: 0pt)

--- page-bleed paged ---
#set page(
  width: 3cm,
  height: 3cm,
  margin: 1cm,
  bleed: 1cm,
  background: rect(width: 100%, height: 100%, fill: green),
)

#context {
  place(
    center + horizon,
    rect(width: page.width, height: page.height, fill: red),
  )
  place(
    center + horizon,
    rect(width: 100%, height: 100%, fill: blue),
  )
}

--- page-bleed-binding paged ---
#set page(
  bleed: (inside: 10pt, outside: 5pt, top: 5pt, bottom: 5pt),
  margin: (outside: 10pt, inside: 5pt, top: 10pt, bottom: 10pt),
)

#rect(width: 100%)
#pagebreak()
#rect(width: 100%)

--- page-bleed-outer-auto eval ---
// Ensure that the auto support from `margin` does not affect `bleed`.
// Error: 18-22 expected relative length or dictionary, found auto
#set page(bleed: auto)

--- page-bleed-inner-auto eval ---
// Error: 18-30 expected relative length, found auto
#set page(bleed: (left: auto))
//@@ file tests/suite/layout_pagebreak.typ
// Test forced page breaks.

--- pagebreak paged ---
// Just a pagebreak.
// Should result in two pages.
#pagebreak()

--- pagebreak-around-set-page paged ---
// Pagebreak, empty with styles and then pagebreak
// Should result in one auto-sized page and two conifer-colored 2cm wide pages.
#pagebreak()
#set page(width: 2cm, fill: conifer)
#pagebreak()

--- pagebreak-weak-after-set-page paged ---
// Two text bodies separated with and surrounded by weak pagebreaks.
// Should result in two aqua-colored pages.
#set page(fill: aqua)
#pagebreak(weak: true)
First
#pagebreak(weak: true)
Second
#pagebreak(weak: true)

--- pagebreak-set-page-mixed paged ---
// Test a combination of pagebreaks, styled pages and pages with bodies.
// Should result in three five pages, with the fourth one being forest-colored.
#set page(width: 80pt, height: 30pt)
#[#set page(width: 60pt); First]
#pagebreak()
#pagebreak()
Third
#page(height: 20pt, fill: forest)[]
Fif#[#set page();th]

--- pagebreak-followed-by-page-call paged ---
// Test hard and weak pagebreak followed by page with body.
// Should result in three navy-colored pages.
#set page(fill: navy)
#set text(fill: white)
First
#pagebreak()
#page[Second]
#pagebreak(weak: true)
#page[Third]

--- pagebreak-in-container paged ---
#box[
  // Error: 4-15 pagebreaks are not allowed inside of containers
  // Hint: 4-15 try using a `#colbreak()` instead
  #pagebreak()
]

--- pagebreak-weak-place paged ---
// After place
// Should result in three pages.
First
#pagebreak(weak: true)
#place(right)[placed A]
#pagebreak(weak: true)
Third

--- pagebreak-weak-meta paged ---
// After only ignorables & invisibles
// Should result in two pages.
First
#pagebreak(weak: true)
#counter(page).update(1)
#metadata("Some")
#pagebreak(weak: true)
Second

--- pagebreak-meta paged ---
// After only ignorables, but regular break
// Should result in three pages.
First
#pagebreak()
#counter(page).update(1)
#metadata("Some")
#pagebreak()
Third

--- pagebreak-to paged ---
#set page(width: 80pt, height: 30pt)
First
#pagebreak(to: "odd")
Third
#pagebreak(to: "even")
Fourth
#pagebreak(to: "even")
Sixth
#pagebreak()
Seventh
#pagebreak(to: "odd")
#page[Ninth]

--- pagebreak-to-auto-sized paged ---
#set page(width: auto, height: auto)

// Test with auto-sized page.
First
#pagebreak(to: "odd")
Third

--- pagebreak-to-multiple-pages paged ---
#set page(height: 30pt, width: 80pt)

// Test when content extends to more than one page
First

Second

#pagebreak(to: "odd")

Third

--- issue-2134-pagebreak-bibliography paged ---
// Test weak pagebreak before bibliography.
#pagebreak(weak: true)
#bibliography("/assets/bib/works.bib")

--- issue-2095-pagebreak-numbering paged ---
// The empty page 2 should not have a page number
#set page(numbering: none)
This and next page should not be numbered

#pagebreak(weak: true, to: "odd")

#set page(numbering: "1")
#counter(page).update(1)

This page should

--- issue-2162-pagebreak-set-style paged ---
// The styles should not be applied to the pagebreak empty page,
// it should only be applied after that.
#pagebreak(to: "even") // We should now skip to page 2

Some text on page 2

#pagebreak(to: "even") // We should now skip to page 4

#set page(fill: orange) // This sets the color of the page starting from page 4
Some text on page 4

--- issue-2591-single-weak-pagebreak paged empty ---
#pagebreak(weak: true)

--- issue-2841-pagebreak-to-weak paged ---
First
#pagebreak(to: "odd")
#pagebreak(weak: true)
Odd
//@@ file tests/suite/layout_relative.typ
--- relative-fields eval ---
// Test relative length fields.
#test((100% + 2em + 2pt).ratio, 100%)
#test((100% + 2em + 2pt).length, 2em + 2pt)
#test((100% + 2pt).length, 2pt)
#test((100% + 2pt - 2pt).length, 0pt)
#test((56% + 2pt - 56%).ratio, 0%)

--- double-percent-embedded eval ---
// Test for two percent signs in a row.
// Error: 2-7 invalid number suffix: `%%`
#3.1%%

--- double-percent-parens eval ---
// Error: 3-8 invalid number suffix: `%%`
#(3.1%%)
//@@ file tests/suite/layout_repeat.typ
// Test the `repeat` function.

--- repeat-basic paged ---
// Test multiple repeats.
#let sections = (
  ("Introduction", 1),
  ("Approach", 1),
  ("Evaluation", 3),
  ("Discussion", 15),
  ("Related Work", 16),
  ("Conclusion", 253),
)

#for section in sections [
  #section.at(0) #box(width: 1fr, repeat[.]) #section.at(1) \
]

--- repeat-dots-rtl paged ---
// Test dots with RTL.
#set text(lang: "ar", font: ("Libertinus Serif", "Noto Sans Arabic"))
مقدمة #box(width: 1fr, repeat[.]) 15

--- repeat-empty paged ---
// Test empty repeat.
A #box(width: 1fr, repeat[]) B

--- repeat-unboxed paged ---
// Test unboxed repeat.
#repeat(rect(width: 2em, height: 1em))

--- repeat-align-and-dir paged ---
// Test single repeat in both directions.
A#box(width: 1fr, repeat(rect(width: 6em, height: 0.7em)))B

#set align(center)
A#box(width: 1fr, repeat(rect(width: 6em, height: 0.7em)))B

#set text(dir: rtl, font: "Noto Sans Arabic")
ريجين#box(width: 1fr, repeat(rect(width: 4em, height: 0.7em)))سون

--- repeat-unrestricted paged ---
#set page(width: auto)
// Error: 2-13 repeat with no size restrictions
#repeat(".")

--- repeat-gap paged ---
// Test repeat with custom gap.
A#box(width: 1fr, repeat(rect(width: 2em, height: 1em), gap: 1em))B

--- repeat-no-justify paged ---
// Test repeat with disabled justification.
#set repeat(justify: false)
A#box(width: 1fr, repeat(rect(width: 2em, height: 1em), gap: 1em))B

--- repeat-no-justify-align paged ---
// Test repeat with alignment and disabled justification.
#set repeat(justify: false)
#set align(right)
A#box(width: 1fr, repeat(rect(width: 2em, height: 1em), gap: 1em))B
//@@ file tests/suite/layout_spacing.typ
// Test the `h` and `v` functions.

--- spacing-h-and-v paged ---
// Linebreak and leading-sized weak spacing are equivalent.
#box[A \ B] #box[A #v(0.65em, weak: true) B]

// Eating up soft spacing.
Inv#h(0pt)isible

// Multiple spacings in a row.
Add #h(10pt) #h(10pt) up

// Relative to area.
#let x = 25% - 4pt
|#h(x)|#h(x)|#h(x)|#h(x)|

// Fractional.
| #h(1fr) | #h(2fr) | #h(1fr) |

--- spacing-rtl paged ---
// Test RTL spacing.
#set text(dir: rtl)
A #h(10pt) B \
A #h(1fr) B

--- spacing-weak-versus-block-spacing paged ---
// Weak spacing wins against block spacing.
0
#v(1cm, weak: true)
#block(above: 2cm, below: 0pt, height: 0pt)
1
#v(1cm, weak: true)
2

--- spacing-fr-weak-collapse paged ---
// Fractional weak spacing should collapse like rel weak spacing.
#set page(height: 100pt)
0
#v(1fr, weak: true)
#v(1fr, weak: false)
1
#v(1fr, weak: false)

--- spacing-fr-weak-standalone-start paged ---
// Standalone fractional weak spacing should collapse.
#set page(height: 60pt)
#v(1fr, weak: true)
0

--- spacing-fr-weak-standalone-end paged ---
// Standalone factional weak spacing at the end should collapse..
#set page(height: 60pt)
#v(1fr, weak: false)
1
#v(1fr, weak: true)

--- spacing-fr-weak-destructs-rel-abs paged ---
// Fr spacing destructs weak rel/abs spacing.
#set page(height: 100pt)
0
#v(1fr, weak: true)
#v(1em, weak: true)
1
#v(1fr, weak: true)
#v(2em, weak: true)
2

--- spacing-fr-weak-destructs-smaller paged ---
// Larger fr destructs smaller fr.
#set page(height: 100pt)
0
#v(1fr, weak: true)
#v(2fr, weak: true) // wins
2
#v(2fr, weak: true)
#v(4fr, weak: true) // wins
6

--- spacing-fr-weak-survives-with-strong paged ---
// Weak fr survives with strong fr, like weak rel survives with strong rel.
#set page(height: 100pt)
#v(1fr, weak: false)
#v(1fr, weak: true)
2
#v(1fr, weak: false)
#v(2fr, weak: true)
4

--- spacing-fr-weak-with-fr-block paged ---
#set page(height: 150pt)
0
#v(2fr, weak: true)
2
#v(1fr, weak: true)
#block(spacing: 0pt, height: 1fr, fill: aqua)[A]
#v(4fr, weak: true)
8

--- spacing-fr-weak-versus-fr-block-spacing paged ---
// Weak fr spacing wins against fr block spacing, just like for weak rel
// spacing.
#set page(height: 100pt)
0
#v(1fr, weak: true)
#block(above: 2fr, below: 0pt, height: 0pt)
1
#v(1fr, weak: true)
2

--- spacing-fr-h-negative paged ---
1 #h(2fr) 2 #h(-1fr) 3 #h(2fr) 4 \
1 #h(-2fr) 2 #h(1fr) 3 #h(-2fr) 4

--- spacing-fr-v-negative paged ---
#set page(height: 100pt, width: 40pt)
#grid(
  columns: (1fr, 1fr),
  align: (left, right),
  [1 #v(2fr) 2 #v(-1fr) 3 #v(2fr) 4],
  [1 #v(-2fr) 2 #v(1fr) 3 #v(-2fr) 4],
)

--- spacing-fr-overflow paged ---
// When the region is already overflowing, fr spacing should resolve relative to
// 0pt rather than the negative remaining space.
#set page(height: 40pt)
#v(1fr)
#rect(height: 60pt)

--- spacing-missing-amount eval ---
// Missing spacing.
// Error: 10-13 missing argument: amount
Totally #h() ignored

--- issue-3624-spacing-behaviour paged ---
// Test that metadata after spacing does not force a new paragraph.
#{
  h(1em)
  counter(heading).update(4)
  [Hello ]
  context counter(heading).display()
}

--- trim-weak-space-line-beginning paged ---
// Weak space at the beginning should be removed.
#h(2cm, weak: true) Hello

--- trim-weak-space-line-end paged ---
// Weak space at the end of the line should be removed.
#set align(right)
Hello #h(2cm, weak: true)

--- issue-4087 paged ---
// Weak space at the end of the line is removed.
This is the first line #h(2cm, weak: true) A new line

// Non-weak space consumes a specified width and pushes to next line.
This is the first line #h(2cm, weak: false) A new line

// Similarly, weak space at the beginning of the line is removed.
This is the first line \ #h(2cm, weak: true) A new line

// Non-weak-spacing, on the other hand, is not removed.
This is the first line \ #h(2cm, weak: false) A new line

--- issue-5244-consecutive-weak-space paged ---
#set par(linebreaks: "optimized")
#{
  [A]
  h(0.3em, weak: true)
  h(0.3em, weak: true)
  [B]
}

--- issue-5244-consecutive-weak-space-heading paged ---
#set par(justify: true)
#set heading(numbering: "I.")

= #h(0.3em, weak: true) test

--- issue-5253-consecutive-weak-space-math paged ---
$= thin thin$ a
//@@ file tests/suite/layout_stack.typ
// Test stack layouts.

--- stack-basic paged ---
// Test stacks with different directions.
#let widths = (
  30pt, 20pt, 40pt, 15pt,
  30pt, 50%, 20pt, 100%,
)

#let shaded(i, w) = {
  let v = (i + 1) * 10%
  rect(width: w, height: 10pt, fill: rgb(v, v, v))
}

#let items = for (i, w) in widths.enumerate() {
  (align(right, shaded(i, w)),)
}

#set page(width: 50pt, margin: 0pt)
#stack(dir: btt, ..items)

--- stack-spacing paged ---
// Test spacing.
#set page(width: 50pt, margin: 0pt)

#let x = square(size: 10pt, fill: eastern)
#stack(
  spacing: 5pt,
  stack(dir: rtl, spacing: 5pt, x, x, x),
  stack(dir: ltr, x, 20%, x, 20%, x),
  stack(dir: ltr, spacing: 5pt, x, x, 7pt, 3pt, x),
)

--- stack-overflow paged ---
// Test overflow.
#set page(width: 50pt, height: 30pt, margin: 0pt)
#box(stack(
  rect(width: 40pt, height: 20pt, fill: conifer),
  rect(width: 30pt, height: 13pt, fill: forest),
))

--- stack-fr paged ---
#set page(height: 3.5cm)
#stack(
  dir: ltr,
  spacing: 1fr,
  ..for c in "ABCDEFGHI" {([#c],)}
)

Hello
#v(2fr)
from #h(1fr) the #h(1fr) wonderful
#v(1fr)
World! 🌍

--- stack-fr-negative paged ---
#set page(height: 150pt, width: 100pt)

#let items = ([1], 2fr, [2], -1fr, [3], 2fr, [4])

#stack(dir: ltr, ..items)
#stack(dir: rtl, ..items)

#grid(
  columns: (1fr, 1fr),
  align: (left, right),
  stack(dir: ttb, ..items),
  stack(dir: btt, ..items),
)

--- stack-rtl-align-and-fr paged ---
// Test aligning things in RTL stack with align function & fr units.
#set page(width: 50pt, margin: 5pt)
#set block(spacing: 5pt)
#set text(8pt)
#stack(dir: rtl, 1fr, [A], 1fr, [B], [C])
#stack(dir: rtl,
  align(center, [A]),
  align(left, [B]),
  [C],
)

--- issue-1240-stack-h-fr paged ---
// This issue is sort of horrible: When you write `h(1fr)` in a `stack` instead
// of directly `1fr`, things go awry. To fix this, we now transparently detect
// h/v children.
#stack(dir: ltr, [a], 1fr, [b], 1fr, [c])
#stack(dir: ltr, [a], h(1fr), [b], h(1fr), [c])

--- issue-1240-stack-v-fr paged ---
#set page(height: 60pt)
#stack(
  dir: ltr,
  spacing: 1fr,
  stack([a], 1fr, [b]),
  stack([a], v(1fr), [b]),
)

--- issue-1918-stack-with-infinite-spacing paged ---
// https://github.com/typst/typst/issues/1918
#set page(width: auto)
#context layout(available => {
  let infinite-length = available.width
  // Error: 3-40 stack spacing is infinite
  stack(spacing: infinite-length)[A][B]
})
//@@ file tests/suite/layout_table.typ
// Test tables.

--- table-empty paged empty ---
#table()

--- table-newlines paged ---
#set page(height: 70pt)
#set table(fill: (x, y) => if calc.even(x + y) { rgb("aaa") })

#table(
  columns: (1fr,) * 3,
  stroke: 2pt + rgb("333"),
  [A], [B], [C], [], [], [D \ E \ F \ \ \ G], [H],
)

--- table-fill-basic paged ---
#table(columns: 3, stroke: none, fill: green, [A], [B], [C])

--- table-fill-bad eval ---
// Error: 14-19 expected color, gradient, tiling, none, array, or function, found string
#table(fill: "hey")

--- table-align-array paged ---
// Test alignment with array.
#table(
  columns: (1fr, 1fr, 1fr),
  align: (left, center, right),
  [A], [B], [C]
)

// Test empty array.
#set align(center)
#table(
  columns: (1fr, 1fr, 1fr),
  align: (),
  [A], [B], [C]
)

--- table-inset paged ---
// Test inset.
#table(
  columns: 3,
  inset: 10pt,
  [A], [B], [C]
)

#table(
  columns: 3,
  inset: (y: 10pt),
  [A], [B], [C]
)

#table(
  columns: 3,
  inset: (left: 20pt, rest: 10pt),
  [A], [B], [C]
)

#table(
  columns: 2,
  inset: (
    left: 20pt,
    right: 5pt,
    top: 10pt,
    bottom: 3pt,
  ),
  [A],
  [B],
)

#table(
  columns: 3,
  fill: (x, y) => (if y == 0 { aqua } else { orange }).darken(x * 15%),
  inset: (x, y) => (left: if x == 0 { 0pt } else { 5pt }, right: if x == 0 { 5pt } else { 0pt }, y: if y == 0 { 0pt } else { 5pt }),
  [A], [B], [C],
  [A], [B], [C],
)

#table(
  columns: 3,
  inset: (0pt, 5pt, 10pt),
  fill: (x, _) => aqua.darken(x * 15%),
  [A], [B], [C],
)

--- table-inset-fold paged ---
// Test inset folding
#set table(inset: 10pt)
#set table(inset: (left: 0pt))

#table(
  fill: red,
  inset: (right: 0pt),
  table.cell(inset: (top: 0pt))[a]
)

--- table-gutters paged ---
// Test interaction with gutters.
#table(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

#table(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  row-gutter: 5pt,
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

#table(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  column-gutter: 5pt,
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

#table(
  columns: (3em, 3em),
  fill: (x, y) => (red, blue).at(calc.rem(x, 2)),
  align: (x, y) => (left, right).at(calc.rem(y, 2)),
  gutter: 5pt,
  [A], [B],
  [C], [D],
  [E], [F],
  [G], [H]
)

--- table-contextual-measurement paged ---
// Test that table cells with varying contextual results are properly
// measured.
#let c = counter("c")
#let k = context square(width: c.get().first() * 5pt)
#let u(n) = [#n] + c.update(n)
#table(
  columns: 3,
  u(1), k, u(2),
  k, u(4), k,
  k, k, k,
)

--- table-header-citation paged ---
#set page(height: 60pt)
#table(
  table.header[@netwok],
  [A],
  [A],
)

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- table-header-counter paged ---
#set page(height: 60pt)
#let c = counter("c")
#table(
  table.header(c.step() + context c.display()),
  [A],
  [A],
)

--- table-header-footer-madness paged ---
#set page(height: 100pt)
#let c = counter("c")
#let it = context c.get().first() * v(10pt)
#table(
  table.header(c.step()),
  [A],
  [A],
  [A],
  [A],
  [A],
  [A],
  [A],
  table.footer(it),
)

--- table-cell-override paged ---
// Cell override
#table(
  align: left,
  fill: red,
  stroke: blue,
  columns: 2,
  [AAAAA], [BBBBB],
  [A], [B],
  table.cell(align: right)[C], [D],
  align(right)[E], [F],
  align(horizon)[G], [A\ A\ A],
  table.cell(align: horizon)[G2], [A\ A\ A],
  table.cell(inset: 0pt)[I], [F],
  [H], table.cell(fill: blue)[J]
)

--- table-cell-show paged ---
// Cell show rule
#show table.cell: it => [Zz]

#table(
  align: left,
  fill: red,
  stroke: blue,
  columns: 2,
  [AAAAA], [BBBBB],
  [A], [B],
  table.cell(align: right)[C], [D],
  align(right)[E], [F],
  align(horizon)[G], [A\ A\ A]
)

--- table-cell-show-and-override paged ---
#show table.cell: it => (it.align, it.fill)
#table(
  align: left,
  row-gutter: 5pt,
  [A],
  table.cell(align: right)[B],
  table.cell(fill: aqua)[B],
)

--- table-cell-set paged ---
// Cell set rules
#set table.cell(align: center)
#show table.cell: it => (it.align, it.fill, it.inset)
#set table.cell(inset: 20pt)
#table(
  align: left,
  row-gutter: 5pt,
  [A],
  table.cell(align: right)[B],
  table.cell(fill: aqua)[B],
)

--- table-cell-folding paged ---
// Test folding per-cell properties (align and inset)
#table(
  columns: (1fr, 1fr),
  rows: (2.5em, auto),
  align: right,
  fill: (x, y) => (green, aqua).at(calc.rem(x + y, 2)),
  [Top], table.cell(align: bottom)[Bot],
  table.cell(inset: (bottom: 0pt))[Bot], table.cell(inset: (bottom: 0pt))[Bot]
)

--- table-cell-align-override paged ---
// Test overriding outside alignment
#set align(bottom + right)
#table(
  columns: (1fr, 1fr),
  rows: 2em,
  align: auto,
  fill: green,
  [BR], [BR],
  table.cell(align: left, fill: aqua)[BL], table.cell(align: top, fill: red.lighten(50%))[TR]
)

--- table-cell-various-overrides paged ---
#table(
  columns: 2,
  fill: green,
  align: right,
  [*Name*], [*Data*],
  table.cell(fill: blue)[J.], [Organizer],
  table.cell(align: center)[K.], [Leader],
  [M.], table.cell(inset: 0pt)[Player]
)

--- table-cell-show-emph paged ---
#{
  show table.cell: emph
  table(
    columns: 2,
    [Person], [Animal],
    [John], [Dog]
  )
}

--- table-cell-show-based-on-position paged ---
// Style based on position
#{
  show table.cell: it => {
    if it.y == 0 {
      strong(it)
    } else if it.x == 1 {
      emph(it)
    } else {
      it
    }
  }
  table(
    columns: 3,
    gutter: 3pt,
    [Name], [Age], [Info],
    [John], [52], [Nice],
    [Mary], [50], [Cool],
    [Jake], [49], [Epic]
  )
}

--- table-cell-par paged ---
// Ensure that table cells aren't considered paragraphs by default.
#show par: highlight

#table(
  columns: 3,
  [A],
  block[B],
  par[C],
)

--- grid-cell-in-table eval ---
// Error: 8-19 cannot use `grid.cell` as a table cell
// Hint: 8-19 use `table.cell` instead
#table(grid.cell[])

--- issue-183-table-lines paged ---
// Ensure no empty lines before a table that doesn't fit into the first page.
#set page(height: 50pt)

Hello
#table(
  columns: 4,
  [1], [2], [3], [4]
)

--- issue-1388-table-row-missing paged ---
// Test that a table row isn't wrongly treated like a gutter row.
#set page(height: 70pt)
#table(
  rows: 16pt,
  ..range(6).map(str).flatten(),
)
//@@ file tests/suite/layout_transform.typ
// Test transformations.

--- transform-tex-logo paged ---
// Test creating the TeX and XeTeX logos.
#let size = 11pt
#let tex = {
  [T]
  h(-0.14 * size)
  box(move(dy: 0.22 * size)[E])
  h(-0.12 * size)
  [X]
}

#let xetex = {
  [X]
  h(-0.14 * size)
  box(scale(x: -100%, move(dy: 0.26 * size)[E]))
  h(-0.14 * size)
  [T]
  h(-0.14 * size)
  box(move(dy: 0.26 * size)[E])
  h(-0.12 * size)
  [X]
}

#set text(font: "New Computer Modern", size)
Neither #tex, \
nor #xetex!

--- transform-rotate-and-scale paged ---
// Test combination of scaling and rotation.
#set page(height: 80pt)
#align(center + horizon,
  rotate(20deg, scale(70%, image("/assets/images/tiger.jpg")))
)

--- transform-rotate-origin paged ---
// Test setting rotation origin.
#rotate(10deg, origin: top + left,
  image("/assets/images/tiger.jpg", width: 50%)
)

--- transform-scale-origin paged ---
// Test setting scaling origin.
#let r = rect(width: 100pt, height: 10pt, fill: forest)
#set page(height: 65pt)
#box(scale(r, x: 50%, y: 200%, origin: left + top))
#box(scale(r, x: 50%, origin: center))
#box(scale(r, x: 50%, y: 200%, origin: right + bottom))

--- transform-rotate paged ---
// Test that rotation impact layout.
#set page(width: 200pt)
#set rotate(reflow: true)

#let one(angle) = box(fill: aqua, rotate(angle)[Test Text])
#for angle in range(0, 360, step: 15) {
  one(angle * 1deg)
}

--- transform-rotate-180deg paged ---
// Test that rotation impact layout.
#set page(width: 200pt)
#set rotate(reflow: true)

#let one(angle) = box(fill: aqua, rotate(angle)[Test Text\ Test Text])
#one(0deg)
#one(180deg)

- #one(0deg)
- #one(180deg)

--- transform-rotate-relative-sizing paged ---
// Test relative sizing in rotated boxes.
#set page(width: 200pt, height: 200pt)
#set text(size: 32pt)
#let rotated(body) = box(rotate(
  90deg,
  box(stroke: 0.5pt, height: 20%, clip: true, body)
))

#set rotate(reflow: false)
Hello #rotated[World]!\

#set rotate(reflow: true)
Hello #rotated[World]!

--- transform-scale paged ---
// Test that scaling impacts layout.
#set page(width: 200pt)
#set text(size: 32pt)
#let scaled(body) = box(scale(
  x: 20%,
  y: 40%,
  body
))

#set scale(reflow: false)
Hello #scaled[World]!

#set scale(reflow: true)
Hello #scaled[World]!

--- transform-scale-relative-sizing paged ---
// Test relative sizing in scaled boxes.
#set page(width: 200pt, height: 200pt)
#set text(size: 32pt)
#let scaled(body) = box(scale(
  x: 60%,
  y: 40%,
  box(stroke: 0.5pt, width: 30%, clip: true, body)
))

#set scale(reflow: false)
Hello #scaled[World]!\

#set scale(reflow: true)
Hello #scaled[World]!

--- transform-scale-abs-and-auto paged ---
// Test scaling by absolute lengths and auto.
#set page(width: 200pt, height: 200pt)
#let cylinder = image("/assets/images/cylinder.svg")

#cylinder
#scale(x: 100pt, y: 50pt, reflow: true, cylinder)
#scale(x: auto, y: 50pt, reflow: true, cylinder)
#scale(x: 100pt, y: auto, reflow: true, cylinder)
#scale(x: 150%, y: auto, reflow: true, cylinder)

--- transform-skew paged ---
// Test skewing along one axis.
#set page(width: 100pt, height: 60pt)
#set text(size: 12pt)
#let skewed(body) = box(skew(ax: -30deg, body))

#set skew(reflow: false)
Hello #skewed[World]!

#set skew(reflow: true)
Hello #skewed[World]!

--- transform-skew-both-axes paged ---
// Test skewing along both axes.
#set page(width: 100pt, height: 250pt)
#set text(size: 12pt)
#let skewed(angle) = box(skew(ax: 30deg, ay: angle)[Some Text])

#set skew(reflow: true)
#for angle in range(-30, 31, step: 10) {
  skewed(angle * 1deg)
}

--- transform-skew-origin paged ---
// Test setting skewing origin.
#set page(width: 100pt, height:40pt)
#set text(spacing: 20pt)
#let square = square.with(width: 8pt)
#let skew-square(origin) = box(place(square(stroke: gray))
  + place(skew(ax: -30deg, ay: -30deg, origin: origin, square())))
#skew-square(center+horizon)
#skew-square(bottom+left)
#skew-square(top+right)
#skew-square(horizon+right)

--- transform-skew-relative-sizing paged ---
// Test relative sizing in skewed boxes.
#set page(width: 100pt, height: 60pt)
#set text(size: 12pt)
#let skewed(body) = box(skew(
  ax: 30deg,
  box(stroke: 0.5pt, width: 30%, clip: true, body)
))

#set skew(reflow: false)
Hello #skewed[World]!\

#set skew(reflow: true)
Hello #skewed[World]!

--- transform-combinations paged ---
#block[
  #rect(width: 10pt, height: 10pt)
  #block(inset: 10pt)[
    #rect(width: 10pt, height: 10pt)
    #rotate(45deg, block(inset: 10pt)[
      #block(inset: 10pt)[
        #rect(width: 10pt, height: 10pt)
        Hello world
        #rect(width: 10pt, height: 10pt, radius: 10pt)
        #rotate(45deg, block(inset: 10pt)[
          #rect(width: 10pt, height: 10pt, radius: 10pt)
          #rect(width: 10pt, height: 10pt, radius: 10pt)
        ])
      ]
    ])
  ]
]

--- issue-7872-negative-scale paged ---
#scale(x: -100%, y: -101%)[hey]
#scale(x: -100%, y: -100%)[hey]
#set text(size: -1em)
#scale(x: -100%, y: -100%)[hey]
//@@ file tests/suite/loading_cbor.typ
--- cbor-decode-integer eval ---
#import "edge-case.typ": cbor-integers
#let data = cbor(cbor-integers)

#for (name, result) in data.representable {
  assert.eq(
    type(result),
    int,
    message: "failed to decode " + name,
  )
}

#for (name, result) in data.large {
  assert.eq(
    type(result),
    float,
    message: "failed to approximately decode " + name,
  )
}

--- cbor-encode-bytes eval ---
#let value = bytes("Typst")
#test(cbor(cbor.encode(value)), value)

--- cbor-encode-any eval ---
#import "edge-case.typ": special-types
#for value in special-types {
  test(
    cbor.encode(value),
    cbor.encode(repr(value)),
  )
}

--- cbor-load-error eval ---
// Error: 7-30 failed to parse CBOR (IO error: failed to fill whole buffer in assets/data/bad.cbor)
#cbor("/assets/data/bad.cbor")
//@@ file tests/suite/loading_csv.typ
--- csv paged ---
// Test reading CSV data.
#set page(width: auto)
#let data = csv("/assets/data/zoo.csv")
#let cells = data.at(0).map(strong) + data.slice(1).flatten()
#table(columns: data.at(0).len(), ..cells)

// Test reading through path type.
#let data-from-path = csv(path("/assets/data/zoo.csv"))
#test(data-from-path, data)

--- csv-row-type-dict eval ---
// Test reading CSV data with dictionary rows enabled.
#let data = csv("/assets/data/zoo.csv", row-type: dictionary)
#test(data.len(), 3)
#test(data.at(0).Name, "Debby")
#test(data.at(2).Weight, "150kg")
#test(data.at(1).Species, "Tiger")

--- csv-file-not-found eval ---
// Error: 6-16 file not found (searched at tests/suite/loading/nope.csv)
#csv("nope.csv")

--- csv-invalid eval ---
// Error: "/assets/data/bad.csv" 3:1 failed to parse CSV (found 3 instead of 2 fields in line 3)
#csv("/assets/data/bad.csv")

--- csv-invalid-row-type-dict eval ---
// Test error numbering with dictionary rows.
// Error: "/assets/data/bad.csv" 3:1 failed to parse CSV (found 3 instead of 2 fields in line 3)
#csv("/assets/data/bad.csv", row-type: dictionary)

--- csv-invalid-delimiter eval ---
// Error: 41-51 delimiter must be an ASCII character
#csv("/assets/data/zoo.csv", delimiter: "\u{2008}")
//@@ file tests/suite/loading_edge-case.typ
// SKIP
// Edge cases in serialization and deserialization.
// They can be imported when testing decoding functions.

#let representable-integer = (
  i64-max: "9223372036854775807",
  i64-min: "-9223372036854775808",
)

#let large-integer = (
  i64-max-plus-one: "9223372036854775808",
  i64-min-minus-one: "-9223372036854775809",
  u64-max: "18446744073709551615",
  u64-max-plus-one: "18446744073709551616",
  i128-max: "170141183460469231731687303715884105727",
  i128-min: "-170141183460469231731687303715884105728",
  u128-max: "340282366920938463463374607431768211455",
)

// The following is generated from the rust script in PR 6836.
#let cbor-integers = bytes(
  (162, 109, 114, 101, 112, 114, 101, 115, 101, 110, 116, 97, 98, 108, 101, 162, 103, 105, 54, 52, 95, 109, 97, 120, 27, 127, 255, 255, 255, 255, 255, 255, 255, 103, 105, 54, 52, 95, 109, 105, 110, 59, 127, 255, 255, 255, 255, 255, 255, 255, 101, 108, 97, 114, 103, 101, 167, 112, 105, 54, 52, 95, 109, 97, 120, 95, 112, 108, 117, 115, 95, 111, 110, 101, 27, 128, 0, 0, 0, 0, 0, 0, 0, 113, 105, 54, 52, 95, 109, 105, 110, 95, 109, 105, 110, 117, 115, 95, 111, 110, 101, 59, 128, 0, 0, 0, 0, 0, 0, 0, 103, 117, 54, 52, 95, 109, 97, 120, 27, 255, 255, 255, 255, 255, 255, 255, 255, 112, 117, 54, 52, 95, 109, 97, 120, 95, 112, 108, 117, 115, 95, 111, 110, 101, 194, 73, 1, 0, 0, 0, 0, 0, 0, 0, 0, 104, 105, 49, 50, 56, 95, 109, 97, 120, 194, 80,
127, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 104, 105, 49, 50, 56, 95, 109, 105, 110, 195, 80, 127, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 104, 117, 49, 50, 56, 95, 109, 97, 120, 194, 80, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255),
)

// These special Typst types are supported by neither human-readable nor binary
// data formats. They should fall back to strings via `repr` when encoded, but
// the specific output can be changed.
#let special-types = (
  decimal("2.99792458"),
  3.14159pt,
  2.99em,
  <sec:intro>,
  panic,
  x => x + 1,
  regex("\\p{Letter}"),
  stroke(red + 5pt),
  auto,
)

// These special Typst types are not supported by human-readable data formats.
#let special-types-for-human = special-types + (bytes("Typst"),)
//@@ file tests/suite/loading_json.typ
--- json eval ---
// Test reading JSON data.
#let data = json("/assets/data/zoo.json")
#test(data.len(), 3)
#test(data.at(0).name, "Debby")
#test(data.at(2).weight, 150)

// Test reading through path type.
#let data-from-path = json(path("/assets/data/zoo.json"))
#test(data-from-path, data)

--- json-with-bom eval ---
// Error: 7-43 failed to parse JSON (unexpected Byte Order Mark at 1:1)
// Hint: 7-43 JSON requires UTF-8 without a BOM
#json(bytes("\u{FEFF}{\"name\": \"BOM\"}"))

--- json-invalid eval ---
// Error: "/assets/data/bad.json" 3:14 failed to parse JSON (expected value at line 3 column 14)
#json("/assets/data/bad.json")

--- issue-3363-json-large-number paged ---
// Big numbers (larger than what i64 can store) should just lose some precision
// but not overflow
#let bignum = json("/assets/data/big-number.json")
#bignum

--- json-decode-number eval ---
#import "edge-case.typ": large-integer, representable-integer

#for (name, source) in representable-integer {
  assert.eq(
    type(json(bytes(source))),
    int,
    message: "failed to decode " + name,
  )
}

#for (name, source) in large-integer {
  assert.eq(
    type(json(bytes(source))),
    float,
    message: "failed to approximately decode " + name,
  )
}

--- json-encode-any eval ---
#import "edge-case.typ": special-types-for-human
#for value in special-types-for-human {
  test(
    json.encode(value),
    json.encode(repr(value)),
  )
}
//@@ file tests/suite/loading_read.typ
--- read-text eval ---
// Test reading plain text files
#let data = read("/assets/text/hello.txt")
#test(data, "Hello, world!\n")

--- read-file-not-found eval ---
// Error: 18-44 file not found (searched at assets/text/missing.txt)
#let data = read("/assets/text/missing.txt")

--- read-invalid-utf-8 eval ---
// Error: 18-40 failed to convert to string (file is not valid UTF-8 in assets/text/bad.txt:1:1)
#let data = read("/assets/text/bad.txt")

--- read-escapes eval ---
// Error: 7-29 path `"../../../../file.txt"` would escape the project root
// Hint: 7-29 cannot access files outside of the project sandbox
// Hint: 7-29 you can adjust the project root with the `--root` argument
#read("../../../../file.txt")

--- read-through-package eval ---
#import "@test/reader:0.1.0"

// Reads from the package.
#test(read(reader.hello-path), "Hello from package\n")
#test(reader.read-it("/hello.txt"), "Hello from package\n")

// Reads from the project.
#test(reader.read-it(path("/assets/text/hello.txt")), "Hello, world!\n")

--- read-from-project-in-package-fails eval ---
#import "@test/reader:0.1.0"

// Error: "tests/packages/reader-0.1.0/src/lib.typ" 2:24-2:25 file not found (searched at tests/packages/reader-0.1.0/assets/text/hello.txt)
#reader.read-it("/assets/text/hello.txt")
//@@ file tests/suite/loading_toml.typ
--- toml eval ---
// Test reading TOML data.
#let data = toml("/assets/data/toml-types.toml")
#test(data.string, "wonderful")
#test(data.integer, 42)
#test(data.float, 3.14)
#test(data.boolean, true)
#test(data.array, (1, "string", 3.0, false))
#test(data.inline_table, ("first": "amazing", "second": "greater") )
#test(data.table.element, 5)
#test(data.table.others, (false, "indeed", 7))
#test(data.date_time, datetime(
  year: 2023,
  month: 2,
  day: 1,
  hour: 15,
  minute: 38,
  second: 57,
))
#test(data.date_time2, datetime(
  year: 2023,
  month: 2,
  day: 1,
  hour: 15,
  minute: 38,
  second: 57,
))
#test(data.date, datetime(
  year: 2023,
  month: 2,
  day: 1,
))
#test(data.time, datetime(
  hour: 15,
  minute: 38,
  second: 57,
))

// Test reading through path type.
#let data-from-path = toml(path("/assets/data/toml-types.toml"))
#test(data-from-path, data)

--- toml-invalid eval ---
// Error: "/assets/data/bad.toml" 1:16-2:1 failed to parse TOML (expected `.`, `=`)
#toml("/assets/data/bad.toml")

--- toml-decode-integer eval ---
#import "edge-case.typ": representable-integer

#for (name, source) in representable-integer {
  assert.eq(
    // The `key` trick is necessary because a TOML documents must be a table.
    type(toml(bytes("key = " + source)).key),
    int,
    message: "failed to decode " + name,
  )
}

--- toml-decode-integer-too-large eval ---
// If an integer cannot be represented losslessly, an error must be thrown.
// https://toml.io/en/v1.0.0#integer

#import "edge-case.typ": large-integer
// Error: 7-55 failed to parse TOML (number too large to fit in target type at 1:7)
#toml(bytes("key = " + large-integer.i64-max-plus-one))

--- toml-decode-integer-too-small eval ---
#import "edge-case.typ": large-integer
// Error: 7-56 failed to parse TOML (number too small to fit in target type at 1:7)
#toml(bytes("key = " + large-integer.i64-min-minus-one))

--- toml-encode-any eval ---
#import "edge-case.typ": special-types-for-human
#for value in special-types-for-human {
  test(
    toml.encode((key: value)),
    toml.encode((key: repr(value))),
  )
}

--- toml-encode-non-table eval ---
// Error: 14-15 expected dictionary, found integer
#toml.encode(3)

--- toml-decode-non-table eval ---
// Error: 7-17 failed to parse TOML (expected `.`, `=` at 1:2)
#toml(bytes("3"))
//@@ file tests/suite/loading_xml.typ
--- xml eval ---
// Test reading XML data.
#let data = xml("/assets/data/hello.xml")
#test(data, ((
  namespace: none,
  tag: "data",
  attrs: (:),
  children: (
    "\n  ",
    (namespace: none, tag: "hello", attrs: (name: "hi"), children: ("1",)),
    "\n  ",
    (
      namespace: none,
      tag: "data",
      attrs: (:),
      children: (
        "\n    ",
        (namespace: none, tag: "hello", attrs: (:), children: ("World",)),
        "\n    ",
        (namespace: none, tag: "hello", attrs: (:), children: ("World",)),
        "\n  ",
      ),
    ),
    "\n",
  ),
),))

// Test reading through path type.
#let data-from-path = xml(path("/assets/data/hello.xml"))
#test(data-from-path, data)

--- xml-namespaces eval ---
// Test reading XML data containing namespaces.
#test(
  xml(bytes(
    ```xml
    <data xmlns="http://example.org" xmlns:foo="urn:foo">
      <hello name="hi">1</hello>
      <foo:hello>World</foo:hello>
    </data>
    ```.text
  )),
  ((
    namespace: "http://example.org",
    tag: "data",
    attrs: (:),
    children: (
      "\n  ",
      (namespace: "http://example.org", tag: "hello", attrs: (name: "hi"), children: ("1",)),
      "\n  ",
      (namespace: "urn:foo", tag: "hello", attrs: (:), children: ("World",)),
      "\n",
    ),
  ),),
)

--- xml-invalid eval ---
// Error: "/assets/data/bad.xml" 3:1 failed to parse XML (found closing tag 'data' instead of 'hello')
#xml("/assets/data/bad.xml")
//@@ file tests/suite/loading_yaml.typ
--- yaml eval ---
// Test reading YAML data
#let data = yaml("/assets/data/yaml-types.yaml")
#test(data.len(), 9)
#test(data.null_key, (none, none))
#test(data.string, "text")
#test(data.integer, 5)
#test(data.float, 1.12)
#test(data.mapping, ("1": "one", "2": "two"))
#test(data.seq, (1,2,3,4))
#test(data.bool, false)
#test(data.keys().contains("true"), true)
#test(data.at("1"), "ok")

// Test reading through path type.
#let data-from-path = yaml(path("/assets/data/yaml-types.yaml"))
#test(data-from-path, data)

--- yaml-invalid eval ---
// Error: "/assets/data/bad.yaml" 2:1 failed to parse YAML (did not find expected ',' or ']' at line 2 column 1, while parsing a flow sequence at line 1 column 18)
#yaml("/assets/data/bad.yaml")

--- yaml-decode-number eval ---
#import "edge-case.typ": large-integer, representable-integer

#for (name, source) in representable-integer {
  assert.eq(
    type(yaml(bytes(source))),
    int,
    message: "failed to decode " + name,
  )
}

#for (name, source) in large-integer {
  assert.eq(
    type(yaml(bytes(source))),
    float,
    message: "failed to approximately decode " + name,
  )
}

--- yaml-encode-any eval ---
#import "edge-case.typ": special-types-for-human
#for value in special-types-for-human {
  test(
    yaml.encode(value),
    yaml.encode(repr(value)),
  )
}
//@@ file tests/suite/math_accent.typ
// Test math accents.

--- math-accent-sym-call paged html ---
// Test function call.
$ grave(a), acute(b), hat(f), tilde(§), macron(ä), dash(ä), breve(ä), \
  dot(!), dot.double(a), diaer(a), dot.triple(a), dot.quad(a), circle(a), \
  acute.double(a), caron(@), arrow(Z), arrow.l(Z), arrow.l.r(Z), \
  harpoon(a), harpoon.lt(a) $

--- math-accent-sym-call-error eval ---
#test(math.tilde(math.pi), math.accent(math.pi, math.tilde))
// Error: 2-9 symbol π is not callable
#math.pi(math.tilde)

--- math-accent-align paged html ---
$ x &= p \ dot(x) &= v \ dot.double(x) &= a \ dot.triple(x) &= j \ dot.quad(x) &= s $

--- math-accent-func paged html ---
// Test `accent` function.
$accent(ö, .), accent(v, <-), accent(ZZ, \u{0303})$

--- math-accent-bounds paged html ---
// Test accent bounds.
$sqrt(tilde(T)) + hat(f)/hat(g)$

--- math-accent-wide-base paged html ---
// Test wide base.
$arrow("ABC" + d), tilde(sum)$

--- math-accent-superscript paged html ---
// Test effect of accent on superscript.
$A^x != hat(A)^x != hat(hat(A))^x$

--- math-accent-high-base paged html ---
// Test high base.
$ tilde(integral), tilde(integral)_a^b, tilde(integral_a^b) $

--- math-accent-sized paged html ---
// Test accent size.
$tilde(sum), tilde(sum, size: #50%), accent(H, hat, size: #200%)$

--- math-accent-sized-script paged html ---
// Test accent size in script size.
$tilde(U, size: #1.1em), x^tilde(U, size: #1.1em), sscript(tilde(U, size: #1.1em))$

--- math-accent-dotless paged html ---
// Test dotless glyph variants.
#let test(c) = $grave(#c), acute(sans(#c)), hat(frak(#c)), tilde(mono(#c)),
  macron(bb(#c)), dot(cal(#c)), diaer(upright(#c)), breve(bold(#c)),
  circle(bold(upright(#c))), caron(upright(sans(#c))), arrow(bold(frak(#c)))$
$test(i) \ test(j)$

--- math-accent-dotless-disabled paged html ---
// Test disabling the dotless glyph variants.
$hat(i), hat(i, dotless: #false), accent(j, tilde), accent(j, tilde, dotless: #false)$

--- math-accent-dotless-set-rule paged html ---
#set math.accent(dotless: false)
$ hat(i) $

--- math-accent-dotless-greedy paged html ---
// Currently the dotless style propagates to everything in the accent's base,
// even though it shouldn't.
$ arrow(P_(c, i dot j) P_(1, i) j) \
  arrow(P_(c, i dot j) P_(1, i) j, dotless: #false) $

--- math-accent-flattened paged html ---
// Test flattened accent glyph variants.
#show math.equation: set text(font: "STIX Two Math")
$hat(a) hat(A)$
$tilde(w) tilde(W)$
$grave(i) grave(j)$
$grave(I) grave(J)$

--- math-accent-flattened-fallback paged html ---
// Test falling back from flattened accent glyph variants when stretching.
#show math.equation: set text(font: "STIX Two Math")
$ hat(A, size: #2em) quad hat(A) quad hat(A, size: #200%) \
  grave(I, size: #2em) quad grave(I) quad grave(I, size: #200%) $

--- math-accent-bottom paged html ---
// Test bottom accents.
$accent(a, \u{20EE}), accent(T, \u{0323}), accent(xi, \u{0332}),
  accent(f, \u{20ED}), accent(F, \u{20E8}), accent(y, \u{032E}),
  accent(!, \u{032F}), accent(J, \u{0333}), accent(p, \u{0331})$

--- math-accent-bottom-wide-base paged html ---
// Test wide base with bottom accents.
$accent(x + y, \u{20EF}), accent(sum, \u{032D})$

--- math-accent-bottom-subscript paged html ---
// Test effect of bottom accent on subscript.
$q_x != accent(q, \u{032C})_x != accent(accent(q, \u{032C}), \u{032C})_x$

--- math-accent-bottom-high-base paged html ---
// Test high base with bottom accents.
$ accent(integral, \u{20EC}), accent(integral, \u{20EC})_a^b, accent(integral_a^b, \u{20EC}) $

--- math-accent-bottom-sized paged html ---
// Test bottom accent size.
$accent(sum, \u{0330}), accent(sum, \u{0330}, size: #50%), accent(H, \u{032D}, size: #200%)$

--- math-accent-cramped paged html ---
// Test whether cramped styles applied to the base of top accents.
$ hat(x^2) x^2 $
$ breve(scripts(sum)^X^X) scripts(sum)^X^X $

--- math-accent-bottom-cramped paged html ---
// Test whether cramped styles applied to the base of bottom accents.
$ accent(x^2, \u{0330}) x^2 $
$ accent(scripts(sum)^X^X, \u{032D}) scripts(sum)^X^X $

--- math-accent-nested paged html ---
// Test nested top and bottom accents.
$hat(accent(L, \u{0330})), accent(circle(p), \u{0323}),
  macron(accent(caron(accent(A, \u{20ED})), \u{0333})) \
  breve(accent(eta, \u{032E})) = accent(breve(eta), \u{032E})$

--- math-accent-string-too-long eval ---
// Error: 17-21 expected exactly one character
$ accent(x + y, "..") $

--- math-accent-content-too-long eval ---
// Error: 17-19 expected a single-codepoint symbol
$ accent(x + y, ..) $

--- math-accent-overlay paged html ---
// Ensure accent is laid out above the base.
#show "\u{0338}": set text(red)
$accent(W, \u{0338})$, $accent(y, \u{0338})$

--- issue-7437-math-accent-text-presentation paged html ---
// Make sure that the `arrow.l.r` symbol correctly works as an accent even
// though it includes a text presentation variation selector.

// Ensure that symbol style works.
$ accent(x + y, arrow.l.r) $
// Ensure that string style works.
$ accent(x + y, "↔") $
// Ensure that content style works.
$ accent(x + y, ↔) $
// Ensure that shorthand style works.
$ accent(x + y, <->) $
// Ensure that function call works.
$ arrow.l.r(x + y) $

--- issue-7437-math-accent-emoji-presentation eval ---
// Check that we do not normalize an accent character with emoji presentation
// variation selector to an accent.
//
// Since we already support arbitrary characters as accents, we might want to
// support clusters like this one, too, but it should render as an emoji instead
// of normalizing into the same accent as `arrow.l.r`, so it's better to keep
// this an error for now.

// Error: 12-31 expected exactly one character
$accent(A, std.emoji.arrow.l.r)$

--- issue-7437-math-accent-trailing-text eval ---
// Test that we don't allow extra text after the text variation selector.

// Error: 13-47 expected exactly one character
$accent(A, #symbol("\u{2194}\u{fe0e}\u{fe0f}"))$
//@@ file tests/suite/math_alignment.typ
// Test implicit alignment math.

--- math-align-weird paged html ---
// Test alignment step functions.
#show: it => context {
  set page(width: 225pt) if target() == "paged"
  it
}
$
a &= c \
  &= c + 1 & "By definition" \
  &= d + 100 + 1000 \
  &= x && "Even longer" \
$

--- math-align-post-fix paged html ---
// Test post-fix alignment.
$
& "right" \
"a very long line" \
"left" \
$

--- math-align-implicit paged html ---
// Test no alignment.
$
"right" \
"a very long line" \
"left" \
$

--- math-align-toggle paged html ---
// Test #460 equations.
$
a &=b & quad c&=d \
e &=f & g&=h
$

--- issue-3973-math-equation-align paged html ---
// In this bug, the alignment set with "show math.equation: set align(...)"
// overrides the left-right alternating behavior of alignment points.
#let equations = [
$ a + b &= c \
      e &= f + g + h $
$         a &= b + c \
  e + f + g &= h $
]
#equations

#show math.equation: set align(start)
#equations

#show math.equation: set align(end)
#equations
//@@ file tests/suite/math_attach.typ
// Test t and b attachments, part 1.

--- math-attach-postscripts paged html ---
// Test basics, postscripts.
$f_x + t^b + V_1^2 + attach(A, t: alpha, b: beta)$

--- math-attach-prescripts paged html ---
// Test basics, prescripts. Notably, the upper and lower prescripts' content need to be
// aligned on the right edge of their bounding boxes, not on the left as in postscripts.
$
attach(upright(O), bl: 8, tl: 16, br: 2, tr: 2-),
attach("Pb", bl: 82, tl: 207) + attach(upright(e), bl: -1, tl: 0) + macron(v)_e \
$

--- math-attach-mixed paged html ---
// A mixture of attachment positioning schemes.
$
attach(a, tl: u),   attach(a, tr: v),   attach(a, bl: x),
attach(a, br: y),   limits(a)^t,        limits(a)_b \

attach(a, tr: v, t: t),
attach(a, tr: v, br: y),
attach(a, br: y, b: b),
attach(limits(a), b: b, bl: x),
attach(a, tl: u, bl: x),
attach(limits(a), t: t, tl: u) \

attach(a, tl: u, tr: v),
attach(limits(a), t: t, br: y),
attach(limits(a), b: b, tr: v),
attach(a, bl: x, br: y),
attach(limits(a), b: b, tl: u),
attach(limits(a), t: t, bl: u),
limits(a)^t_b \

attach(a, tl: u, tr: v, bl: x, br: y),
attach(limits(a), t: t, bl: x, br: y, b: b),
attach(limits(a), t: t, tl: u, tr: v, b: b),
attach(limits(a), tl: u, bl: x, t: t, b: b),
attach(limits(a), t: t, b: b, tr: v, br: y),
attach(a, tl: u, t: t, tr: v, bl: x, b: b, br: y)
$

--- math-attach-followed-by-func-call paged html ---
// Test function call after subscript.
$pi_1(Y), a_f(x), a^zeta (x), a^abs(b)_sqrt(c) \
 a^subset.eq (x), a_(zeta(x)), pi_(1(Y)), a^(abs(b))_(sqrt(c))$

--- math-attach-nested paged html ---
// Test associativity and scaling.
$ 1/(V^2^3^4^5),
  frac(
    attach(
      limits(V), br: attach(2, br: 3), b: attach(limits(2), b: 3)),
    attach(
      limits(V), tl: attach(2, tl: 3), t: attach(limits(2), t: 3))),
  attach(Omega,
    tl: attach(2, tl: attach(3, tl: attach(4, tl: 5))),
    tr: attach(2, tr: attach(3, tr: attach(4, tr: 5))),
    bl: attach(2, bl: attach(3, bl: attach(4, bl: 5))),
    br: attach(2, br: attach(3, br: attach(4, br: 5))),
  )
$

--- math-attach-high paged html ---
// Test high subscript and superscript.
$ sqrt(a_(1/2)^zeta), sqrt(a_alpha^(1/2)), sqrt(a_(1/2)^(3/4)) \
  sqrt(attach(a, tl: 1/2, bl: 3/4)),
  sqrt(attach(a, tl: 1/2, bl: 3/4, tr: 1/2, br: 3/4)) $

--- math-attach-descender-collision paged html ---
// Test for no collisions between descenders/ascenders and attachments.

$ sup_(x in P_i) quad inf_(x in P_i) $
$ op("fff",limits: #true)^(y) quad op("yyy", limits:#true)_(f) $

--- math-attach-to-group paged html ---
// Test frame base.
$ (-1)^n + (1/2 + 3)^(-1/2) $

--- math-attach-horizontal-align paged html ---
#set text(size: 8pt)

// Test that the attachments are aligned horizontally.
$ x_1 p_1 frak(p)_1 2_1 dot_1 lg_1 !_1 \\_1 ]_1 "ip"_1 op("iq")_1 \
  x^1 b^1 frak(b)^1 2^1 dot^1 lg^1 !^1 \\^1 ]^1 "ib"^1 op("id")^1 \
  "_"_1 "`"^1 x_1 y_1 x^1 l^1 attach(I,tl:1,bl:1,tr:1,br:1)
  scripts(sum)_1^1 integral_1^1 abs(1/2)_1^1 \
  x^1_1, ")"^1_1 (b y)^1_1, "[∫]"_1 [integral]_1 $

--- math-attach-limit paged html ---
// Test limit.
$ lim_(n->oo \ n "grows") sum_(k=0 \ k in NN)^n k $

--- math-attach-force-scripts-and-limits paged html ---
// Test forcing scripts and limits.
$ limits(A)_1^2 != A_1^2 $
$ scripts(sum)_1^2 != sum_1^2 $
$ limits(integral)_a^b != integral_a^b $

--- issue-math-attach-realize-panic paged html ---
// Error: 25-29 unknown variable `oops`
$ attach(A, t: #context oops) $

--- math-attach-show-limit paged html ---
// Show and let rules for limits and scripts
#let eq = $ ∫_a^b iota_a^b $
#eq
#show "∫": math.limits
#show math.iota: math.limits.with(inline: false)
#eq
$iota_a^b$

--- math-attach-default-placement paged html ---
// Test default of limit attachments on relations at all sizes.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$ a =^"def" b quad a lt.eq_"really" b quad  a arrow.r.long.squiggly^"slowly" b $
$a =^"def" b quad a lt.eq_"really" b quad a arrow.r.long.squiggly^"slowly" b$

$a scripts(=)^"def" b quad a scripts(lt.eq)_"really" b quad a scripts(arrow.r.long.squiggly)^"slowly" b$

--- math-attach-integral paged html ---
// Test default of scripts attachments on integrals at display size.
$ integral.inter_a^b  quad \u{2a1b}_a^b quad limits(\u{2a1b})_a^b $
$integral.inter_a^b quad \u{2a1b}_a^b quad limits(\u{2a1b})_a^b$

--- math-attach-large-operator paged html ---
// Test default of limit attachments on large operators at display size only.
$ tack.t.big_0^1 quad \u{02A0A}_0^1 quad bowtie.big_0^1 $
$tack.t.big_0^1 quad \u{02A0A}_0^1 quad bowtie.big_0^1$

--- math-attach-limit-long paged html ---
// Test long limit attachments.
$ attach(product, t: 123456789) attach(product, t: 123456789, bl: x) \
  attach(product, b: 123456789) attach(product, b: 123456789, tr: x) $
$attach(limits(product), t: 123456789) attach(limits(product), t: 123456789, bl: x)$

$attach(limits(product), b: 123456789) attach(limits(product), b: 123456789, tr: x)$

--- math-attach-kerning paged html ---
// Test math kerning.
#show math.equation: set text(font: "STIX Two Math")

$ L^A Y^c R^2 delta^y omega^f a^2 t^w gamma^V p^+ \
  b_lambda f_k p_i x_1 x_j x_A y_l y_y beta_s theta_k \
  J_0 Y_0 T_1 T_f V_a V_A F_j cal(F)_j lambda_y \
  attach(W, tl: l) attach(A, tl: 2) attach(cal(V), tl: beta)
  attach(cal(P), tl: iota) attach(f, bl: i) attach(A, bl: x)
  attach(cal(J), bl: xi) attach(cal(A), bl: m) $

--- math-attach-kerning-mixed paged html ---
// Test mixtures of math kerning.
#show math.equation: set text(font: "STIX Two Math")

$ x_1^i x_2^lambda x_2^(2alpha) x_2^(k+1) x_2^(-p_(-1)) x_j^gamma \
  f_2^2 v_0^2  z_0^2 beta_s^2 xi_i^k J_1^2 N_(k y)^(-1) V_pi^x \
  attach(J, tl: 1, br: i) attach(P, tl: i, br: 2) B_i_0 phi.alt_i_(n-1)
  attach(A, tr: x, bl: x, br: x, tl: x) attach(F, tl: i, tr: f) \
  attach(cal(A), tl: 2, bl: o) attach(cal(J), bl: l, br: A)
  attach(cal(y), tr: p, bl: n t) attach(cal(O), tl: 16, tr: +, br: sigma)
  attach(italic(Upsilon), tr: s, br: Psi, bl: d) $

--- math-attach-nested-base paged html ---
// Test attachments when the base has attachments.
$ attach(a^b, b: c) quad
  attach(attach(attach(attach(attach(attach(sum, tl: 1), t: 2), tr: 3), br: 4), b: 5), bl: 6) $

#let a0 = math.attach(math.alpha, b: [0])
#let a1 = $alpha^1$
#let a2 = $attach(a1, bl: 3)$

$ a0 + a1 + a0_2 \
  a1_2 + a0^2 + a1^2 \
  a2 + a2_2 + a2^2 $

--- math-attach-nested-base-bad-placement paged html ---
// This used to have bad placement with nested attachments.
$
  // `b:2` in the middle attach used to stop `b:4` from moving inward.
  attach(attach(attach(a, t: 1), b: 2), t: 3, b: 4)
  quad
  attach(attach(b, t: 1, b: 2), t: 3, b: 4)
$

--- math-attach-nested-deep-base paged html ---
// Test attachments when the base has attachments and is nested arbitrarily
// deep.
#{
  let var = $x^1$
  for i in range(24) {
    var = $var$
  }
  $var_2$
}

--- math-attach-scripts-extended-shapes paged html ---
// Test script attachments positioning if the base is an extended shape (or a
// sequence of extended shapes).
$lr(size: #240%, [x])_0^1, [x]_0^1, \]_0^1, x_0^1, A_0^1$ \
$n^2, (n + 1)^2, sum_0^1, integral_0^1$

--- math-attach-missing-sides eval ---
// Test attachments that are missing a side.
// Error: 23-24 unexpected underscore
$ a _ b (d _) (d'_ ) (_ c) $

--- math-attach-double-chain paged html ---
// Test attachment chain grouping with doubled operators and primes
$ mat(delim: #none,
  a_1_2^3,  b^1^2_3,  c_1^2^3,  d^1_2_3;
  a'_1_2^3, b'^1^2_3, c'_1^2^3, d'^1_2_3;
  a_1'_2^3, b^1'^2_3, c_1'^2^3, d^1'_2_3;
) $
//@@ file tests/suite/math_call.typ
// Test function calls in math.

// Tests for two-dimensional arguments in matrices are in `mat.typ`.

// Tests for method calls in math are in `../scripting/methods.typ` as
// `math-field-call-*`.

--- math-call-non-func paged html ---
// Using call syntax with a non-function in math renders the callee next to
// parens by "unparsing" the arguments into content.
$ phi(x) $
$ phi(x, y, 1/2) $

--- math-call-non-func-empty-args paged html ---
// Trailing commas and empty args introduce blank content in math
$ sin(,x,y,,,) $
// with whitespace/trivia:
$ sin( ,/**/x/**/, , /**/y, ,/**/, ) $

--- math-call-non-func-named eval ---
// Hint: 3-6 `phi` is not a function
// Error: 10-18 named-argument syntax can only be used with functions
// Hint: 10-18 to render the colon as text, escape it: `alpha\: y`
$ phi(x, alpha: y) $

--- math-call-non-func-spread eval ---
// Hint: 3-6 `phi` is not a function
// Error: 10-17 spread-argument syntax can only be used with functions
// Hint: 10-17 to render the dots as text, add a space: `.. alpha`
$ phi(x, ..alpha) $

--- math-call-non-func-2d paged html ---
$ pi(a;b) quad gamma(;) quad eta(#"a";;, ; upright(b),) $

--- math-call-non-func-spacing paged html ---
// Test that we keep the same spacing when unparsing.
#show regex("[,;]"): math.class.with("fence")
$ phi(| , | ; |) \
  phi/**/(| , | ; |) $
#test($     phi(| , | ; |) $,
      $ phi/**/(| , | ; |) $)

--- math-call-unclosed-func eval ---
#let func(x) = x
// Error: 6-7 unclosed delimiter
$func(a$

--- math-call-unclosed-non-func eval ---
// Error: 5-6 unclosed delimiter
$sin(x$

--- math-call-missing-operator-sides eval ---
// Error: 6-7 unclosed delimiter
$func(1^2_, 1/; , , √ )$

--- math-call-basic eval ---
// Function arguments in math.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(a)$, "arguments([a])")
#check($args(a,)$, "arguments([a])")
#check($args(a,b)$, "arguments([a], [b])")
#check($args(a,b,)$, "arguments([a], [b])")

--- math-call-empty-args eval ---
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(,x,,y,,)$, "arguments([], [x], [], [y], [])")
// with whitespace/trivia:
#check($args( ,/**/x/**/, , /**/y, ,/**/, )$, "arguments([], [x], [], [y], [], [])")

--- math-call-escape eval ---
// Escaped characters in calls.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(a\;b)$, "arguments(sequence([a], [;], [b]))")
#check($args(a\,b;c)$, "arguments((sequence([a], [,], [b]),), ([c],))")
#check($args(b\;c\,d;e)$, "arguments((sequence([b], [;], [c], [,], [d]),), ([e],))")
#check($args(a\: b)$, "arguments(sequence([a], [:], [ ], [b]))")
#check($args(a : b)$, "arguments(sequence([a], [ ], [:], [ ], [b]))")
#check($args(\..a)$, "arguments(sequence([.], [.], [a]))")
#check($args(.. a)$, "arguments(sequence([.], [.], [ ], [a]))")
#check($args(a..b)$, "arguments(sequence([a], [.], [.], [b]))")

--- math-call-pass-to-box paged ---
// When passing to a function, we lose the italic styling if we wrap the content
// in a non-math function, unless it's already nested in some math element (lr,
// attach, etc.)
//
// This is not good, so this test should fail and be updated once it is fixed.
#let id(body) = body
#let bx(body) = box(body, stroke: blue+0.5pt, inset: (x:2pt, y:3pt))
#let eq(body) = math.equation(body)
$
     x y   &&quad     x (y z)   &quad     x y^z  \
  id(x y)  &&quad  id(x (y z))  &quad  id(x y^z) \
  eq(x y)  &&quad  eq(x (y z))  &quad  eq(x y^z) \
  bx(x y)  &&quad  bx(x (y z))  &quad  bx(x y^z) \
$

--- math-call-named-basic eval ---
// Named arguments in math.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(_a: a)$, "arguments(_a: [a])")
#check($args(_a-b: a)$, "arguments(_a-b: [a])")
#check($args(a-b: a)$, "arguments(a-b: [a])")
#check($args(a-b-c: a)$, "arguments(a-b-c: [a])")
#check($args(a--c: a)$, "arguments(a--c: [a])")
#check($args(a: a-b)$, "arguments(a: sequence([a], [−], [b]))")
#check($args(a: a:b)$, "arguments(a: sequence([a], [:], [b]))")
#check($args(a: a : b)$, "arguments(a: sequence([a], [ ], [:], [ ], [b]))")
#check($args(a-b: a-b)$, "arguments(a-b: sequence([a], [−], [b]))")
#check($args(a-b)$, "arguments(sequence([a], [−], [b]))")

--- math-call-named-no-expr eval ---
#let func(m: none) = m
// Error: 10 expected expression
$ func(m: ) $

--- math-call-named-duplicate eval ---
#let func(my: none) = my
// Error: 15-17 duplicate argument: my
$ func(my: a, my: b) $

--- math-call-named-shorthand-clash-1 eval ---
#let func(m: none) = m
// Error: 18-21 unexpected argument
$func(m: =) func(m:=)$

--- math-call-named-shorthand-clash-2 eval ---
#let func(m: none) = m
// Error: 41-45 unexpected argument
$func(m::) func(m: :=) func(m:: =) func(m::=)$

--- math-call-named-single-underscore eval ---
#let func(x) = x
// Error: 8-9 expected identifier, found underscore
$ func(_: a) $

--- math-call-named-single-char-error eval ---
#let func(m: none) = m
// Error: 8-13 unexpected argument
$ func(m : a) $

--- math-call-spread-basic eval ---
// Spread arguments in math.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(..#range(0, 4).chunks(2))$, "arguments((0, 1), (2, 3))")
#check($#args(range(1, 5).chunks(2))$, "arguments(((1, 2), (3, 4)))")
#check($#args(..range(1, 5).chunks(2))$, "arguments((1, 2), (3, 4))")
#check($args(#(..range(2, 6).chunks(2)))$, "arguments(((2, 3), (4, 5)))")
#let nums = range(0, 4).chunks(2)
#check($args(..nums)$, "arguments((0, 1), (2, 3))")
#check($args(..nums, ..nums)$, "arguments((0, 1), (2, 3), (0, 1), (2, 3))")
#check($args(..nums, 4, 5)$, "arguments((0, 1), (2, 3), [4], [5])")
#check($args(..nums, ..#range(4, 6))$, "arguments((0, 1), (2, 3), 4, 5)")
#check($args(..nums, #range(4, 6))$, "arguments((0, 1), (2, 3), (4, 5))")

--- math-call-spread-empty eval ---
// Test that a spread operator followed by nothing generates two dots.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(..)$, "arguments(sequence([.], [.]))")
#check($args(.., ..; .. , ..)$, "arguments(\n  (sequence([.], [.]), sequence([.], [.])),\n  (sequence([.], [.]), sequence([.], [.])),\n)")

--- math-call-spread-content-error eval ---
#let args(..body) = body
// Error: 7-16 cannot spread content
$args(..(a + b))$

--- math-call-spread-multiple-exprs eval ---
#let args(..body) = body
// Error: 7-14 cannot spread content
$args(..a + b)$

--- math-call-spread-unexpected-dots eval ---
#let args(..body) = body
// Error: 8-10 unexpected dots
$args(#..range(1, 5).chunks(2))$

--- math-call-spread-unexpected-binary eval ---
// Test spread operators followed by binary math operators with and without
// right operands. These errors aren't great, but they can be silenced with a
// space and no one would actually write this.
$
  // Error: 9-10 unexpected slash
  // Error: 19-20 unexpected hat
  // Error: 29-30 unexpected underscore
  vec(../.) vec(..^.) vec(.._.)
  // Error: 9-10 unexpected slash
  // Error: 19-20 unexpected hat
  // Error: 29-30 unexpected underscore
  vec(../)  vec(..^)  vec(.._)
$

--- math-call-spread-shorthand-clash paged html ---
#let func(body) = body
$func(...)$

--- math-call-named-spread-override eval ---
// Test named argument overriding with the spread operator.
#let check(it, s) = test(it.body.text, repr(s))
#let func(a: 1, b: 1) = (a: a, b: b)
#let dict = (a: 2, b: 2)
#let args = arguments(a: 3, b: 3)
#check($func()$, (a: 1, b: 1))
#check($func(..dict, ..args)$, (a: 3, b: 3))
#check($func(..args, ..dict)$, (a: 2, b: 2))
#check($func(a: #4, ..dict, b: #4)$, (a: 2, b: 4))
#check($func(a: #4, ..args, b: #4)$, (a: 3, b: 4))

--- math-call-named-spread-duplicate eval ---
// Test duplicate named args with the spread operator.
// The error should only happen for manually added args.
#let func(..) = none
#let dict = (a: 1)
// Error: 22-23 duplicate argument: a
$func(a: #2, ..dict, a: #3)$

--- math-call-2d-basic eval ---
// Two-dimensional arguments in math. More tests can be found in `mat.typ`.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(a;b)$, "arguments(([a],), ([b],))")
#check($args(a,b;c)$, "arguments(([a], [b]), ([c],))")
#check($args(a,b;c,d;e,f)$, "arguments(([a], [b]), ([c], [d]), ([e], [f]))")
#check($args( a; b; )$, "arguments(([a],), ([b],))")
#check($args(a;  ; c)$, "arguments(([a],), ([],), ([c],))")
#check($args(a b,/**/; b)$, "arguments((sequence([a], [ ], [b]), []), ([b],))")
#check($args(a/**/b, ; b)$, "arguments((sequence([a], [b]), []), ([b],))")
#check($args( ;/**/a/**/b/**/; )$, "arguments(([],), (sequence([a], [b]),))")
#check($args( ; , ; )$, "arguments(([],), ([], []))")
#check($args(/**/; // funky whitespace/trivia
    ,   /**/  ;/**/)$, "arguments(([],), ([], []))")

--- math-call-2d-semicolon-embedded-code eval ---
// If a semicolon directly follows an embedded code expression, it terminates
// the code expression instead of indicating 2d arguments.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#check($args(#false;)$, "arguments(false)")
#check($args("a" #"b";)$, "arguments(sequence([a], [ ], [b]))")
#check($args(#true ;)$, "arguments((true,))")
#check($args(#true;;)$, "arguments((true,))")

--- math-call-2d-named eval ---
// Two-dimensional args with named args.
#let args(..body) = (body.pos(), body.named())
#let check(it, r) = test-repr(it.body.text, r)
#check($args(a: b)$, "((), (a: [b]))")
#check($args(a: b,)$, "((), (a: [b]))")
#check($args(a: b;)$, "(((),), (a: [b]))")
#check($args(1, 2; 3, 4)$, "((([1], [2]), ([3], [4])), (:))")

// This set should all be the same.
#check($args(a: b, 1, 2; 3, 4)$, "((([1], [2]), ([3], [4])), (a: [b]))")
#check($args(1, a: b, 2; 3, 4)$, "((([1], [2]), ([3], [4])), (a: [b]))")
#check($args(1, 2, a: b; 3, 4)$, "((([1], [2]), ([3], [4])), (a: [b]))")
#check($args(1, 2; a: b, 3, 4)$, "((([1], [2]), ([3], [4])), (a: [b]))")
#check($args(1, 2; 3, a: b, 4)$, "((([1], [2]), ([3], [4])), (a: [b]))")
#check($args(1, 2; 3, 4, a: b)$, "((([1], [2]), ([3], [4])), (a: [b]))")
#check($args(1, 2; 3, 4; a: b)$, "((([1], [2]), ([3], [4])), (a: [b]))")

#check($args(a: b, 1, 2, 3, c: d)$, "(([1], [2], [3]), (a: [b], c: [d]))")
#check($args(1, 2, 3; a: b)$, "((([1], [2], [3]),), (a: [b]))")
#check($args(a-b: a,, e:f;; d)$, "((([],), ([],), ([d],)), (a-b: [a], e: [f]))")
#check($args(a: b, ..#range(0, 4))$, "((0, 1, 2, 3), (a: [b]))")

--- math-call-2d-spread-pos eval ---
// Two-dimensional args with positional spreading.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#let nums = range(0, 4).chunks(2)
#check($args(..nums;)$, "arguments(((0, 1), (2, 3)))")
#check($args(..nums; ,)$, "arguments(((0, 1), (2, 3)), ([],))")
#check($args(..nums; ;)$, "arguments(((0, 1), (2, 3)), ([],))")
#check($args(..nums; 1, 2; 3, 4)$, "arguments(((0, 1), (2, 3)), ([1], [2]), ([3], [4]))")
#check($args(..nums, 1, 2; 3, 4)$, "arguments(((0, 1), (2, 3), [1], [2]), ([3], [4]))")
#check($args(1, 2; ..nums)$, "arguments(([1], [2]), ((0, 1), (2, 3)))")
#check($args(1, 2; 3, 4)$, "arguments(([1], [2]), ([3], [4]))")
#check($args(1, 2; 3, 4; ..#range(5, 7))$, "arguments(([1], [2]), ([3], [4]), (5, 6))")
#check($args(1, 2; 3, 4, ..#range(5, 7))$, "arguments(([1], [2]), ([3], [4], 5, 6))")
#check($args(1, 2; 3, 4, ..#range(5, 7);)$, "arguments(([1], [2]), ([3], [4], 5, 6))")
#check($args(1, 2; 3, 4, ..#range(5, 7),)$, "arguments(([1], [2]), ([3], [4], 5, 6))")

--- math-call-2d-spread-named eval ---
// Two-dimensional args with named and positional spreading.
#let args(..body) = body
#let check(it, r) = test-repr(it.body.text, r)
#let nums = range(0, 4).chunks(2)
#let dict = (one: 1, two: 2)
#let both = arguments(..nums, ..dict)
#check($args(..nums;)$, "arguments(((0, 1), (2, 3)))")
#check($args(..dict;)$, "arguments(one: 1, two: 2, ())") // Adds an empty array
#check($args(1, ..dict;)$, "arguments(one: 1, two: 2, ([1],))")
#check($args(1, ..dict, 2;)$, "arguments(one: 1, two: 2, ([1], [2]))")
#check($args(1; ..dict, 2;)$, "arguments(one: 1, two: 2, ([1],), ([2],))")
#check($args(1; ..dict; 2;)$, "arguments(one: 1, two: 2, ([1],), (), ([2],))")
#check($args(..nums, ..dict;)$, "arguments(one: 1, two: 2, ((0, 1), (2, 3)))")
#check($args(..both;)$, "arguments(one: 1, two: 2, ((0, 1), (2, 3)))")
#check($args(..nums; ..dict)$, "arguments(one: 1, two: 2, ((0, 1), (2, 3)))")
#check($args(..dict; ..nums)$, "arguments(one: 1, two: 2, (), ((0, 1), (2, 3)))")

--- issue-3774-math-call-empty-2d-args paged html ---
$ mat(;,) $
// Add some whitespace/trivia:
$ mat(; ,) $
$ mat(;/**/,) $
$ mat(;
,) $
$ mat(;// line comment
,) $
$ mat(
  1, , ;
   ,1, ;
   , ,1;
) $

--- math-call-value-error eval ---
// TODO: We should produce this error for `mat` as well by changing from
// `Value::display()` to `Value::cast::<Content>()`.
$ mat(#1) $
// Error: 8-9 expected content, found integer
$ vec(#1) $

--- math-call-value-non-func eval ---
#test($sin(1)$, $sin(#1)$)

--- issue-2885-math-var-only-in-global eval ---
// Error: 7-10 unknown variable `rgb`
// Hint: 7-10 `rgb` is not available directly in math, but is in the standard library
// Hint: 7-10 to access `rgb` in code mode you can add a hash: `#rgb`
// Hint: 7-10 or access `rgb` in math mode by using the `std` module: `std.rgb`
$text(rgb(0, 0, 0), "foo")$

--- math-call-shadowed-builtin paged html ---
// We don't error if we try to call a shadowed standard library function.
#let box = "box"
$ box() $

--- math-call-error eval ---
// Test the span of errors when calling a function.
#let func(a, b, c) = {}
// Error: 3-13 missing argument: c
$ func(a, b) $

--- math-call-2d-error eval ---
// Test the span of errors for 2d arguments.
// The current range isn't the best, but it's hard to improve.
#let func() = {}
// Error: 7-20 unexpected argument
$ func(a, b; c, d;) $

--- math-call-error-inside-func eval ---
// Test whether errors inside function calls produce further errors.
#let int = int
$ int(
  // Error: 3-8 missing argument: value
  int()
) $
//@@ file tests/suite/math_cancel.typ
// Tests the cancel() function.

--- math-cancel-inline paged ---
// Inline
$a + 5 + cancel(x) + b - cancel(x)$

$c + (a dot.c cancel(b dot.c c))/(cancel(b dot.c c))$

--- math-cancel-display paged ---
// Display
#set page(width: auto)
$ a + b + cancel(b + c) - cancel(b) - cancel(c) - 5 + cancel(6) - cancel(6) $
$ e + (a dot.c cancel((b + c + d)))/(cancel(b + c + d)) $

--- math-cancel-inverted paged ---
// Inverted
$a + cancel(x, inverted: #true) - cancel(x, inverted: #true) + 10 + cancel(y) - cancel(y)$
$ x + cancel("abcdefg", inverted: #true) $

--- math-cancel-cross paged ---
// Cross
$a + cancel(b + c + d, cross: #true, stroke: #red) + e$
$ a + cancel(b + c + d, cross: #true) + e $

--- math-cancel-background paged ---
// Background
$a + cancel(b, background: #true, stroke: #(red + 3pt))$

--- math-cancel-customized paged ---
// Resized and styled
#set page(width: 200pt, height: auto)
$a + cancel(x, length: #200%) - cancel(x, length: #50%, stroke: #(red + 1.1pt))$
$ b + cancel(x, length: #150%) - cancel(a + b + c, length: #50%, stroke: #(blue + 1.2pt)) $

--- math-cancel-angle-absolute paged ---
// Specifying cancel line angle with an absolute angle
$cancel(x, angle: #0deg) + cancel(x, angle: #45deg) + cancel(x, angle: #90deg) + cancel(x, angle: #135deg)$

--- math-cancel-angle-func paged ---
// Specifying cancel line angle with a function
$x + cancel(y, angle: #{angle => angle + 90deg}) - cancel(z, angle: #(angle => angle + 135deg))$
$ e + cancel((j + e)/(f + e)) - cancel((j + e)/(f + e), angle: #(angle => angle + 30deg)) $

--- math-cancel-angle-func-panic paged trace ---
// Error: 29-41 panicked with: 47.92deg
#let compute-angle(angle) = panic(angle)

// Trace: 35-48 (1) while calling `compute-angle`
$ cancel((j + e)/(f + e), angle: #compute-angle) $
//@@ file tests/suite/math_cases.typ
// Test case distinction.

--- math-cases paged html ---
$ f(x, y) := cases(
  1 quad &"if" (x dot y)/2 <= 0,
  2 &"if" x divides 2,
  3 &"if" x in NN,
  4 &"else",
) $

--- math-cases-gap paged html ---
#set math.cases(gap: 1em)
$ x = cases(1, 2) $

--- math-cases-delim paged html ---
#set math.cases(delim: sym.chevron.l)
$ cases(a, b, c) $

--- math-cases-linebreaks paged html ---
// Warning: 40-49 linebreaks are ignored in branches
// Hint: 40-49 use commas instead to separate each line
$ cases(a, b, c) cases(reverse: #true, a \ b \ c) $

--- math-cases-linebreaks-trailing paged html ---
// Warning: 9-12 linebreaks are ignored in branches
// Hint: 9-12 use commas instead to separate each line
$ cases(a \ ) $

--- math-cases-delim-class paged html ---
// Test that delimiters have opening and closing math class.
$ 2cases(a, reverse: #true, delim: bar.v) 2 $
$ 2 cases(a, delim: bar.v)2 $
//@@ file tests/suite/math_class.typ
// Test math classes.

--- math-class-chars paged html ---
// Test characters.
$ a class("normal", +) b \
  a class("binary", .) b \
  lr(class("opening", \/) a/b class("closing", \\)) \
  { x class("fence", \;) x > 0} \
  a class("large", \/) b \
  a class("punctuation", :) b \
  a class("relation", !) b \
  a + class("unary", times) b \
  class("vary", :) a class("vary", :) b $

--- math-class-content paged ---
// Test custom content.
#let dotsq = square(
  size: 0.7em,
  stroke: 0.5pt,
  align(center+horizon, circle(radius: 0.15em, fill: black))
)

$ a dotsq b \
  a class("normal", dotsq) b \
  a class("vary", dotsq) b \
  a + class("vary", dotsq) b \
  a class("punctuation", dotsq) b $

--- math-class-nested paged html ---
// Test nested.
#let normal = math.class.with("normal")
#let pluseq = $class("binary", normal(+) normal(=))$
$ a pluseq 5 $

--- math-class-exceptions paged html ---
// Test exceptions.
$ sqrt(3)\/2 quad d_0.d_1d_2 dots $

--- math-class-limits paged html ---
// Test if the math class changes the limit configuration.
$ class("normal", ->)_a $
$class("relation", x)_a$
$ class("large", x)_a $
$class("large", ->)_a$

$limits(class("normal", ->))_a$
$ scripts(class("relation", x))_a $

--- math-class-not-recursive paged html ---
$ & log x \
  & log (x+y) \
  & log class("normal", (x+y)) $

--- issue-4985-up-tack-is-normal-perp-is-relation paged html ---
$ top = 1 \
  bot = 2 \
  a perp b $
//@@ file tests/suite/math_delimited.typ
// Test delimiter matching and scaling.

--- math-lr-matching paged html ---
// Test automatic matching.
#show: it => context {
  set page(width: 122pt) if target() == "paged"
  it
}
$ (a) + {b/2} + abs(a)/2 + (b) $
$f(x/2) < zeta(c^2 + abs(a + b/2))$

--- math-lr-unmatched paged html ---
// Test unmatched.
$[1,2[ = [1,2) != zeta\(x/2\) $

--- math-lr-call paged html ---
// Test manual matching.
$ [|a/b|] != lr(|]a/b|]) != [a/b) $
$ lr(| ]1,2\[ + 1/2|) $

--- math-lr-fences paged html ---
// Test fence confusion.
$ |x + |y| + z/a| \
  lr(|x + |y| + z/a|) $

--- math-lr-symbol-unmatched paged html ---
// Test that symbols aren't matched automatically.
$ bracket.l a/b bracket.r
  = lr(bracket.l a/b bracket.r) $

--- math-lr-half paged html ---
// Test half LRs.
$ lr(a/b\]) = a = lr(\{a/b) $

--- math-lr-size paged html ---
// Test manual scaling.
$ lr(]sum_(x=1)^n x], size: #70%)
  < lr((1, 2), size: #220%) $

--- math-lr-shorthands paged html ---
// Test predefined delimiter pairings.
$floor(x/2), ceil(x/2), abs(x), norm(x)$

--- math-lr-color paged html ---
// Test colored delimiters
$ lr(
    text(\(, fill: #green) a/b
    text(\), fill: #blue)
  ) $

--- math-lr-mid paged html ---
// Test middle functions
$ { x mid(|) sum_(i=1)^oo phi_i (x) < 1 } \
  { integral |dot|
      mid(bar.v.double)
    floor(hat(I) mid(slash) { dot mid(|) dot } mid(|) I/n) } $

--- math-lr-mid-size paged html ---
// Test mid when lr size is set.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}

$ lr({ A mid(|) integral }) quad
  lr(size: #1em, { A mid(|) integral }) quad
  lr(size: #(1em+20%), { A mid(|) integral }) \

  lr(] A mid(|) integral ]) quad
  lr(size: #1em, ] A mid(|) integral ]) quad
  lr(size: #(1em+20%), ] A mid(|) integral ]) \

  lr(( A mid(|) integral ]) quad
  lr(size: #1em, ( A mid(|) integral ]) quad
  lr(size: #(1em+20%), ( A mid(|) integral ]) $

--- math-lr-mid-size-nested-equation paged html ---
// Test mid size when lr size is set, when nested in an equation.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}

#let body = ${ A mid(|) integral }$
$ lr(body) quad
  lr(size: #1em, body) quad
  lr(size: #(1em+20%), body) $

--- math-lr-mid-class paged html ---
// Test that `mid` creates a Relation, but that can be overridden.
$ (a | b) $
$ (a mid(|) b) $
$ (a class("unary", |) b) $
$ (a class("unary", mid(|)) b) $
$ (a mid(class("unary", |)) b) $

--- math-lr-unbalanced paged html ---
// Test unbalanced delimiters.
$ 1/(2 (x) $
$ 1_(2 y (x) () $
$ 1/(2 y (x) (2(3)) $

--- math-lr-weak-spacing paged html ---
// Test ignoring weak spacing immediately after the opening
// and immediately before the closing.
$ [#h(1em, weak: true)A(dif x, f(x) dif x)sum#h(1em, weak: true)] $
$ lr(\[#h(1em, weak: true)lr(A dif x, f(x) dif x\))sum#h(1em, weak:true)a) $

--- math-lr-nested paged html ---
// Test nested lr calls.
// As there is no body, relative lengths are relative to 0.
#let body1 = math.lr($|$, size: 4em)
#let body2 = $lr(|, size: #4em)$

$lr(|, size: #2em)$
$lr(lr(|, size: #4em), size: #50%)$
$lr(body1, size: #50%)$
$lr(body2, size: #50%)$

--- math-lr-nested-mid paged html ---
// Test nested lr calls with mid.
$ lr(lr({ integral_0^1 x mid(|) 1/e }, size: #3em), size: #50%) $

--- math-lr-ignore-ignorant paged html ---
// Test ignoring leading and trailing ignorant fragments.
#box($ (1 / 2) $)
#box({
  show "(": it => context it
  $ (1 / 2) $
})
#box({
  show ")": it => context it
  $ (1 / 2) $
})
#box({
  show "(": it => context it
  show ")": it => context it
  $ (1 / 2) $
})

--- math-lr-scripts paged html ---
// Test interactions with script attachments.
$ lr(size: #3em, |)_a^b lr(size: #3em, zws|)_a^b
  lr(size: #3em, [x])_0^1 [x]_0^1
  lr(size: #1em, lr(size: #10em, [x]))_0^1 $

--- issue-4188-lr-corner-brackets paged html ---
// Test positioning of U+231C to U+231F
$⌜a⌟⌞b⌝$ = $⌜$$a$$⌟$$⌞$$b$$⌝$

--- math-lr-unparen paged html ---
// Test that unparen with brackets stays as an LrElem.
#let item = $limits(sum)_i$
$
  1 / ([item]) quad
  1 /  [item]
$

--- math-lr-sym-call paged html ---
$
  ceil.l(x) \
  floor.l(x) \

  paren.l(x) \
  paren.l.flat(x) \
  paren.l.closed(x) \
  paren.l.stroked(x) \

  brace.l(x) \
  brace.l.stroked(x) \

  bracket.l(x) \
  bracket.l.tick.t(x) \
  bracket.l.tick.b(x) \
  bracket.l.stroked(x) \

  shell.l(x) \
  shell.l.stroked(x) \
  shell.l.filled(x) \

  bag.l(x) \

  mustache.l(x) \
  mustache.r(x) \

  fence.l(x) \
  fence.l.double(x) \

  chevron.l(x) \
  chevron.l.curly(x) \
  chevron.l.dot(x) \
  chevron.l.closed(x) \
  chevron.l.double(x) \

  corner.l.t(x) \
  corner.l.b(x) \

  bar(x) \
  bar.double(x) \
  bar.triple(x) \
  fence.dotted(x) \
$

--- math-lr-sym-call-size paged html ---
$ bracket.l(x, size: #400%) $

--- math-lr-sym-call-extra-arg eval ---
// Error: 16-28 unexpected argument: nope
$ bracket.l(x, nope: "nope") $

--- math-lr-sym-func-repr eval ---
// The outline thing is just a roundabout way to force a cast from symbol to
// function...
#test(repr(outline(indent: sym.chevron.l.curly).indent), "(..) => ..")

--- math-lr-multiline-trailing paged html ---
$ (a \ ) $

--- math-lr-multiline paged html ---
$ 1 + (a/b + b \ = c) + 2 $
$ abs(x + y & 1 & 2 & a + b \ 3 &&& 4, size: #200%) $

--- math-lr-multiline-align paged html ---
$ (a & b \ c) $
$ (a \ b & c) $

--- math-lr-single-line-align paged html ---
$   & x \
    & x ) $
$   & x \
  ( & x ) $
$   & x \
  ( & x   $
//@@ file tests/suite/math_equation.typ
// Test alignment of block equations.
// Test show rules on equations.

--- math-equation-numbering paged html ---
#show: it => context {
  set page(width: 150pt) if target() == "paged"
  it
}
#set math.equation(numbering: "(I)")

We define $x$ in preparation of @fib:
$ phi.alt := (1 + sqrt(5)) / 2 $ <ratio>

With @ratio, we get
$ F_n = round(1 / sqrt(5) phi.alt^n) $ <fib>

--- math-equation-font paged html ---
// Test different font.
#show math.equation: set text(font: "Noto Sans Math")
$ v := vec(1 + 2, 2 - 4, sqrt(3), arrow(x)) + 1 $

--- math-equation-show-rule paged html ---
This is small: $sum_(i=0)^n$

#show math.equation: math.display
This is big: $sum_(i=0)^n$

--- math-equation-align-unnumbered paged html ---
// Test unnumbered
#let eq(alignment) = {
  show math.equation: set align(alignment)
  $ a + b = c $
}

#eq(center)
#eq(left)
#eq(right)

#set text(dir: rtl)
#eq(start)
#eq(end)

--- math-equation-align-numbered paged html ---
// Test numbered
#let eq(alignment) = {
  show math.equation: set align(alignment)
  $ a + b = c $
}

#set math.equation(numbering: "(1)")

#eq(center)
#eq(left)
#eq(right)

#set text(dir: rtl)
#eq(start)
#eq(end)

--- math-equation-number-align paged html ---
#set math.equation(numbering: "(1)")

$ a + b = c $

#show math.equation: set align(center)
$ a + b = c $
#show math.equation: set align(left)
$ a + b = c $
#show math.equation: set align(right)
$ a + b = c $

#set text(dir: rtl)
#show math.equation: set align(start)
$ a + b = c $
#show math.equation: set align(end)
$ a + b = c $

--- math-equation-number-align-start paged html ---
#set math.equation(numbering: "(1)", number-align: start)

$ a + b = c $

#show math.equation: set align(center)
$ a + b = c $
#show math.equation: set align(left)
$ a + b = c $
#show math.equation: set align(right)
$ a + b = c $

#set text(dir: rtl)
#show math.equation: set align(start)
$ a + b = c $
#show math.equation: set align(end)
$ a + b = c $

--- math-equation-number-align-end paged html ---
#set math.equation(numbering: "(1)", number-align: end)

$ a + b = c $

#show math.equation: set align(center)
$ a + b = c $
#show math.equation: set align(left)
$ a + b = c $
#show math.equation: set align(right)
$ a + b = c $

#set text(dir: rtl)
#show math.equation: set align(start)
$ a + b = c $
#show math.equation: set align(end)
$ a + b = c $

--- math-equation-number-align-left paged html ---
#set math.equation(numbering: "(1)", number-align: left)

$ a + b = c $

#show math.equation: set align(center)
$ a + b = c $
#show math.equation: set align(left)
$ a + b = c $
#show math.equation: set align(right)
$ a + b = c $

#set text(dir: rtl)
#show math.equation: set align(start)
$ a + b = c $
#show math.equation: set align(end)
$ a + b = c $

--- math-equation-number-align-right paged html ---
#set math.equation(numbering: "(1)", number-align: right)

$ a + b = c $

#show math.equation: set align(center)
$ a + b = c $
#show math.equation: set align(left)
$ a + b = c $
#show math.equation: set align(right)
$ a + b = c $

#set text(dir: rtl)
#show math.equation: set align(start)
$ a + b = c $
#show math.equation: set align(end)
$ a + b = c $

--- math-equation-number-align-center eval ---
// Error: 52-58 expected `start`, `left`, `right`, or `end`, found center
#set math.equation(numbering: "(1)", number-align: center)

--- math-equation-number-align-center-bottom eval ---
// Error: 52-67 expected `start`, `left`, `right`, or `end`, found center
#set math.equation(numbering: "(1)", number-align: center + bottom)

--- math-equation-number-align-monoline paged html ---
#set math.equation(numbering: "(1)")
$ p = sum_k k ln a $

#set math.equation(numbering: "(1)", number-align: top)
$ p = sum_k k ln a $

#set math.equation(numbering: "(1)", number-align: bottom)
$ p = sum_k k ln a $

--- math-equation-number-align-multiline paged html ---
#set math.equation(numbering: "(1)")

$ p &= ln a b \
    &= ln a + ln b $

--- math-equation-number-align-multiline-top-start paged html ---
#set math.equation(numbering: "(1)", number-align: top+start)

$ p &= ln a b \
    &= ln a + ln b $
$ q &= sum_k k ln a \
    &= sum_k ln A $

--- math-equation-number-align-multiline-bottom paged html ---
#show math.equation: set align(left)
#set math.equation(numbering: "(1)", number-align: bottom)

$ p &= ln a b \
    &= ln a + ln b $
$ q &= sum_k ln A \
    &= sum_k k ln a $

--- math-equation-number-align-multiline-expand paged html ---
// Tests that if the numbering's layout box vertically exceeds the box of
// the equation frame's boundary, the latter's frame is resized correctly
// to encompass the numbering. #box() below delineates the resized frame.
//
// A row with "-" only has a height that's smaller than the height of the
// numbering's layout box. Note we use pattern "1" here, not "(1)", since
// the parenthesis exceeds the numbering's layout box, due to the default
// settings of top-edge and bottom-edge of the TextElem that laid it out.
#let equations = [
  #box($ - - - $, fill: silver)
  #box(
  $ - - - \
    a = b $,
  fill: silver)
  #box(
  $ a = b \
    - - - $,
  fill: silver)
]

#set math.equation(numbering: "1", number-align: top)
#equations

#set math.equation(numbering: "1", number-align: horizon)
#equations

#set math.equation(numbering: "1", number-align: bottom)
#equations

--- math-equation-number-align-multiline-no-expand paged ---
// Tests that if the numbering's layout box doesn't vertically exceed the
// box of the equation frame's boundary, the latter's frame size remains.
// So, in the grid below, frames in each row should have the same height.
#set math.equation(numbering: "1")
#grid(
  columns: 4 * (1fr,),
  column-gutter: 3 * (2pt,),
  row-gutter: 2pt,
  align: horizon,
  [
    #set math.equation(number-align: horizon)
    #box($ - - \ a \ sum $, fill: silver)
  ],
  [
    #set math.equation(number-align: bottom)
    #box($ - - \ a \ sum $, fill: silver)
  ],
  [
    #set math.equation(number-align: horizon)
    #box($ sum \ a \ - - $, fill: silver)
  ],
  [
    #set math.equation(number-align: top)
    #box($ sum \ a \ - - $, fill: silver)
  ],

  [
    #set math.equation(number-align: horizon)
    #box($ - - $, fill: silver)
  ],
  [
    #set math.equation(number-align: top)
    #box($ - - $, fill: silver)
  ],
  [
    #set math.equation(number-align: bottom)
    #box($ - - $, fill: silver)
  ],
)

--- math-equation-number-empty paged ---
// Test numbering on empty equations.
#math.equation(numbering: "1", block: true, [])

--- math-equation-tag-affects-row-height paged ---
// Tags should not affect the row height of equations.
#set stack(dir: ltr, spacing: 0.5em)
#stack(
  box($ - - $, fill: silver),
  box($ #metadata(none) - - $, fill: silver),
)

#stack(
  box($ a \ - - $, fill: silver),
  box($ a \ #metadata(none) - - $, fill: silver),
  box($ - - \ a $, fill: silver),
  box($ #metadata(none) - - \ a $, fill: silver),
)

--- issue-4187-alignment-point-affects-row-height paged ---
// In this bug, a row of "-" only should have a very small height; but
// after adding an alignment point "&", the row gains a larger height.
// We need to test alignment point "&" does not affect a row's height.
#set stack(dir: ltr, spacing: 0.5em)
#stack(
  box($ - - $, fill: silver),
  box($ - - $, fill: silver)
)

#stack(
  box($ a \ - - $, fill: silver),
  box($ &- - \ &a $, fill: silver),
  box($ &a \ &- - $, fill: silver)
)

--- issue-numbering-hint paged html ---
// In this bug, the hint and error messages for an equation
// being reference mentioned that it was a "heading" and was
// lacking the proper path.
#show: it => context {
  set page(width: 70pt) if target() == "paged"
  it
}

$
    Delta = b^2 - 4 a c
$ <quadratic>

// Error: 14-24 cannot reference equation without numbering
// Hint: 14-24 you can enable equation numbering with `#set math.equation(numbering: "1.")`
Looks at the @quadratic formula.

--- issue-3696-equation-rtl paged html ---
#show: it => context {
  set page(width: 150pt) if target() == "paged"
  it
}
#set text(lang: "he")
תהא סדרה $a_n$: $[a_n: 1, 1/2, 1/3, dots]$

--- issue-6170-equation-stroke paged html ---
// In this bug stroke settings did not apply to math content.
// We expect all of these to have a green stroke.
#set text(stroke: green + 0.5pt)

A $B^2$ $ grave(C)' $
//@@ file tests/suite/math_frac.typ
// Test fractions.

--- math-frac-baseline paged html ---
// Test that denominator baseline matches in the common case.
$ x = 1/2 = a/(a h) = a/a = a/(1/2) $

--- math-frac-paren-removal paged html ---
// Test parenthesis removal.
$ (|x| + |y|)/2 < [1+2]/3 $

--- math-frac-large paged html ---
// Test large fraction.
$ x = (-b plus.minus sqrt(b^2 - 4a c))/(2a) $

--- math-binom paged html ---
// Test binomial.
$ binom(circle, square) $

--- math-binom-multiple paged html ---
// Test multinomial coefficients.
$ binom(n, k_1, k_2, k_3) $

--- math-binom-missing-lower eval ---
// Error: 3-13 missing argument: lower
$ binom(x^2) $

--- math-dif paged html ---
// Test dif.
$ (dif y)/(dif x), dif/x, x/dif, dif/dif \
  frac(dif y, dif x), frac(dif, x), frac(x, dif), frac(dif, dif) $

--- math-frac-associativity paged html ---
// Test associativity.
$ 1/2/3 = (1/2)/3 = 1/(2/3) $

--- math-frac-tan-sin-cos paged html ---
// A nice simple example of a simple trig property.
$ tan(x) = sin(x) / cos(x) \
  tan x = (sin x) / (cos x) $

--- math-frac-precedence paged html ---
// Test precedence.
$ a_1/b_2, 1/f(x), zeta(x)/2, "foo"[|x|]/2 \
  1.2/3.7, 2.3^3.4 \
  f [x]/2, phi [x]/2 \
  +[x]/2, 1(x)/2, 2[x]/2, 🏳️‍🌈[x]/2 \
  (a)b/2, b(a)[b]/2 \
  n!/2, 5!/2, n !/2, 1/n!, 1/5! $

--- math-frac-implicit-func paged html ---
// Test other precedence interactions with implicit function calls.
$
  f'(x) / f_pi{x} \
  sin^2(x) / f_0(x) quad f!(x) / g^(-1)(x) \
  a_\u{2a}[|x} / a_"2a"{x|] quad f_pi.alt{x} / f_#math.pi.alt{x} \
  a(b)_c(d)^e(f) / g(h)'_i(j)' \
  (x)'(x)'(x)' / (x)'(x)'(x)' \
$

--- math-frac-precedence-xid-vs-alpha eval ---
// Many characters have the Unicode `Alphabetic` property but not `xid_start`,
// but only two have `xid_start` but not `Alphabetic` (℮ U+212E and ℘ U+2118).
// We used to use `Alphabetic` to determine whether characters would be treated
// as an implicit function call, but now we use `xid_start`. This is partly
// because the chars with `Alphabetic` but not `xid_start` are not very
// function-like, but mainly because ℘ is an actual named function!
// https://en.wikipedia.org/wiki/Weierstrass_elliptic_function
// https://util.unicode.org/UnicodeJsps/list-unicodeset.jsp?a=\p{alpha}+-+\p{xids}

#let alphabetic = regex("\p{alpha}")
#let xid-start = regex("\p{xids}")

// https://en.wikipedia.org/wiki/Weierstrass_elliptic_function
#assert(not "℘".contains(alphabetic))
#assert("℘".contains(xid-start))
#let eqn = $ ℘()/2  $
#test(repr(eqn.body), "frac(
  num: sequence([℘], lr(body: sequence([(], [)]))),
  denom: [2],
)")

#assert("ⓟ".contains(alphabetic))
#assert(not "ⓟ".contains(xid-start))
#let eqn = $ ⓟ()/2  $
#test(repr(eqn.body), "sequence([ⓟ], frac(num: [], denom: [2]))")

--- math-frac-precedence-non-ident-grapheme eval ---
// This single grapheme starts with a letter, but does not act as the start of
// an identifier due to U+20DD COMBINING ENCLOSING CIRCLE not being part of
// `xid_continue`.
#let eqn = $ A⃝()/2  $
#test(repr(eqn.body), "sequence([A⃝], frac(num: [], denom: [2]))")

--- math-frac-gap paged html ---
// Test that the gap above and below the fraction rule is correct.
$ sqrt(n^(2/3)) $

--- math-frac-horizontal paged html ---
// Test that horizontal fractions look identical to inline math with `slash`
#set math.frac(style: "horizontal")
$ (a / b) / (c / (d / e)) $
$ (a slash b) slash (c slash (d slash e)) $

--- math-frac-horizontal-lr-paren paged ---
// Test that parentheses are in a left-right pair even when rebuilt by a horizontal fraction
#set math.frac(style: "horizontal")
$ (#v(2em)) / n $

--- math-frac-skewed paged ---
// Test skewed fractions
#set math.frac(style: "skewed")
$ a / b,  a / (b / c) $

--- math-frac-horizontal-explicit paged html ---
// Test that explicit fractions don't change parentheses
#set math.frac(style: "horizontal")
$ frac(a, (b + c)), frac(a, b + c) $

--- math-frac-horizontal-nonparen-brackets paged html ---
// Test that non-parentheses left-right pairs remain untouched
#set math.frac(style: "horizontal")
$ [x+y] / {z} $

--- math-frac-styles-inline paged ---
// Test inline layout of styled fractions
#set math.frac(style: "horizontal")
$a/(b+c), frac(a, b+c, style: "skewed"), frac(a, b+c, style: "vertical")$

--- math-frac-line-fill-stroke paged html ---
// Test that the horizontal stroke is also decorated like text glyphs
#text(size: 20pt, fill: yellow, stroke: red + .5pt)[$1/Delta$]
#text(size: 25pt, stroke: red)[$1/Delta$]
//@@ file tests/suite/math_interactions.typ
// Test interactions with styling and normal layout.
// Hint: They are bad ...

--- math-unknown-var-hint eval ---
// Error: 4-6 unknown variable `ab`
// Hint: 4-6 if you meant to display multiple letters as is, try adding spaces between each letter: `a b`
// Hint: 4-6 or if you meant to display this as text, try placing it in quotes: `"ab"`
$ 5ab $

--- math-nested-normal-layout paged ---
// Test images and font fallback.
#let monkey = move(dy: 0.2em, image("/assets/images/monkey.svg", height: 1em))
$ sum_(i=#emoji.apple)^#emoji.apple.red i + monkey/2 $

--- math-table paged ---
// Test tables.
$ x := #table(columns: 2)[x][y]/mat(1, 2, 3)
     = #table[A][B][C] $

--- math-equation-auto-wrapping paged html ---
// Test non-equation math directly in content.
#math.attach($a$, t: [b])

--- math-font-switch paged html ---
// Test font switch.
#let here = text.with(font: "Noto Sans")
$#here[f] := #here[Hi there]$.

--- math-root-show-rule-1 paged html ---
#show "√": set text(red, font: "Noto Sans Math")
$ root(2, (a + b) / c) $

--- math-root-show-rule-2 paged html ---
#show "√": set text(2em)
$ sqrt(2) $

--- math-root-show-rule-3 paged html ---
// Test cursed show rule.
#show "√": "!"
$ sqrt(2) root(2, 2) $

--- math-root-show-rule-4 paged html ---
#show math.root: set text(red)
$ sqrt(x + y) root(4, 2) $

--- math-root-show-rule-5 paged html ---
#show math.root: it => {
  show "√": set text(purple) if it.index == none
  it
}
$ sqrt(1/2) root(3, 1/2) $

--- math-delim-show-rule-1 paged html ---
#show regex("\\[|\\]"): set text(green, font: "Noto Sans Math")
$ mat(delim: \[, a, b, c; d, e, f; g, h, i) quad [x + y] $

--- math-delim-show-rule-2 paged html ---
#show math.vec: it => {
  show regex("\\(|\\)"): set text(blue)
  it
}
$ vec(1, 0, 0), mat(1; 0; 0), (1), binom(n, k) $

--- math-delim-show-rule-3 paged html ---
#show "⏟": set text(fuchsia)
$ underbrace(1 + 1 = 2, "obviously") $

--- math-delim-show-rule-4 paged html ---
#show "{": set text(navy)
$ cases(x + y + z = 0, 2x - y = 0, -5y + 2z = 0) $

--- math-delim-show-rule-5 paged html ---
#show regex("\\(|\\)"): set text(1.5em)
$ 10 dot (9 - 5) dot (1/2 - 1) $

--- math-primes-show-rule paged html ---
#show math.primes: set text(maroon)
$f'(x), f''''''(x)$

--- math-glyph-show-rule paged html ---
#show "+": set text(orange, font: "Noto Sans Math")
$ 1 + 1 = +2 $
#show "+": text(2em)[#sym.plus.o]
$ 1 + 1 = +2 $

--- math-accent-show-rule-1 paged html ---
#show "\u{0302}": set text(blue, font: "XITS Math")
$hat(x)$, $hat(hat(x))$, x\u{0302}

--- math-accent-show-rule-2 paged html ---
#let rhat(x) = {
  show "\u{0302}": set text(red)
  math.hat(x)
}
$hat(x)$, $rhat(x)$, $hat(rhat(x))$, $rhat(hat(x))$, x\u{0302}

--- math-accent-show-rule-3 paged html ---
#show math.accent: it => {
  show "\u{0300}": set text(green)
  it
}
$grave(x)$, x\u{0300}

--- math-accent-show-rule-4 paged ---
#show "\u{0302}": box(baseline: -5pt, text(0.5em, sym.diamond.small))
$hat(X)$, $hat(x)$

--- math-box-without-baseline paged ---
// Test boxes without a baseline act as if the baseline is at the base
#{
  box(stroke: 0.2pt, $a #box(stroke: 0.2pt, $a$)$)
  h(12pt)
  box(stroke: 0.2pt, $a #box(stroke: 0.2pt, $g$)$)
  h(12pt)
  box(stroke: 0.2pt, $g #box(stroke: 0.2pt, $g$)$)
}

--- math-box-with-baseline paged ---
// Test boxes with a baseline are respected
#box(stroke: 0.2pt, $a #box(baseline:0.5em, stroke: 0.2pt, $a$)$)

--- math-at-par-start paged html ---
// Test that equation at start of paragraph works fine.
$x$ is a variable.

--- math-at-par-end paged html ---
// Test that equation at end of paragraph works fine.
One number is $1$

--- math-at-line-start paged ---
// Test math at the natural end of a line.
#h(60pt) Number $1$ exists.

--- math-at-line-end paged ---
// Test math at the natural end of a line.
#h(50pt) Number $1$ exists.

--- math-consecutive paged html ---
// Test immediately consecutive equations.
$x$$y$

--- math-symbol-show-rule paged html ---
// Test using rules for symbols
#show sym.tack: it => $#h(1em) it #h(1em)$
$ a tack b $

--- issue-math-realize-show paged ---
// Test that content in math can be realized without breaking
// nested equations.
#let my = $pi$
#let f1 = box(baseline: 10pt, [f])
#let f2 = context f1
#show math.vec: [nope]

$ pi a $
$ my a $
$ 1 + sqrt(x/2) + sqrt(#hide($x/2$)) $
$ a x #link("url", $+ b$) $
$ f f1 f2 $
$ vec(1,2) * 2 $

--- issue-math-realize-hide paged ---
$ x^2 #hide[$(>= phi.alt) union y^2 0$] z^2 $
Hello #hide[there $x$]
and #hide[$ f(x) := x^2 $]

--- issue-math-realize-scripting paged html ---
// Test equations can embed equation pieces built by functions
#let foo(v1, v2) = {
  // Return an equation piece that would've been rendered in
  // inline style if the piece is not embedded
  $v1 v2^2$
}
#let bar(v1, v2) = {
  // Return an equation piece that would've been rendered in
  // block style if the piece is not embedded
  $ v1 v2^2 $
}
#let baz(..sink) = {
  // Return an equation piece built by joining arrays
  sink.pos().map(x => $hat(#x)$).join(sym.and)
}

Inline $2 foo(alpha, (M+foo(a, b)))$.

Inline $2 bar(alpha, (M+foo(a, b)))$.

Inline $2 baz(x,y,baz(u, v))$.

$ 2 foo(alpha, (M+foo(a, b))) $
$ 2 bar(alpha, (M+foo(a, b))) $
$ 2 baz(x,y,baz(u, v)) $

--- math-size-resolve paged html ---
#let length = context repr(measure("--").width)
$ a length a ^ length $

--- math-size-space paged html ---
$ x space x script(space) x sscript(space) x \
  x^inline(space) x^space x^sscript(space) x $

--- math-size-arbitrary-content paged ---
// Test sizing of both relative and absolute non math content in math sizes.
#let stuff = square(inset: 0pt)[hello]
#let square = square(size: 5pt)
$ stuff sum^stuff_square square $

--- math-size-math-content-1 paged ---
// Nested math content has styles overwritten by the inner equation.
// Ideally the widths would match the actual length of the arrows.
#let arrow = $stretch(->)^"much text"$
$ arrow A^arrow A^A^arrow $
#let width = context measure(arrow).width
$ width A^width A^A^width $

--- math-size-math-content-2 paged ---
// Nested math content has styles overwritten by the inner equation.
// Ideally the heights would match the actual height of the sums.
#let sum = $sum^2$
#let height(x) = context measure(x).height
$sum = height(sum) $
$ sum != height(sum) $

--- math-size-math-content-3 paged ---
// Sum doesn't get wrapped in math as it is a single expr.
// Ideally the height would match the actual height of the sum.
#let height(x) = context measure(x).height
$ sum != height(sum) $

--- math-text-size paged ---
// Values retrieved from function are not resolved at the moment.
// Ideally the left size would match the right size.
#let size = context [#text.size.to-absolute() #1em.to-absolute()]
$ size x^size x^x^size $
//@@ file tests/suite/math_mat.typ
// Test matrices.

--- math-mat-semicolon paged html ---
// Test semicolon syntax.
#set align(center)
$mat() dot
 mat(;) dot
 mat(1, 2) dot
 mat(1, 2;) \
 mat(1; 2) dot
 mat(1, 2; 3, 4) dot
 mat(1 + &2, 1/2; &3, 4)$

--- math-mat-sparse paged html ---
// Test sparse matrix.
$ mat(
  1, 2, ..., 10;
  2, 2, ..., 10;
  dots.v, dots.v, dots.down, dots.v;
  10, 10, ..., 10;
) $

--- math-mat-baseline paged html ---
// Test baseline alignment.
$ mat(
  a, b^2;
  sum_(x \ y) x, a^(1/2);
  zeta, alpha;
) $

--- math-mat-delim-set paged html ---
// Test alternative delimiter with set rule.
#set math.mat(delim: "[")
$ mat(1, 2; 3, 4) $
$ a + mat(delim: #none, 1, 2; 3, 4) + b $

--- math-mat-delim-direct paged ---
// Test alternative math delimiter directly in call.
#set align(center)
#grid(
  columns: 3,
  gutter: 10pt,

  $ mat(1, 2, delim: "[") $,
  $ mat(1, 2; delim: "[") $,
  $ mat(delim: "[", 1, 2) $,

  $ mat(1; 2; delim: "[") $,
  $ mat(1; delim: "[", 2) $,
  $ mat(delim: "[", 1; 2) $,

  $ mat(1, 2; delim: "[", 3, 4) $,
  $ mat(delim: "[", 1, 2; 3, 4) $,
  $ mat(1, 2; 3, 4; delim: "[") $,
)

--- math-mat-spread paged ---
// Test argument spreading in matrix.
$ mat(..#range(1, 5).chunks(2))
  mat(#(..range(2).map(_ => range(2)))) $

#let nums = ((1,) * 5).intersperse(0).chunks(3)
$ mat(..nums, delim: "[") $

--- math-mat-spread-1d paged html ---
$ mat(..#range(1, 5) ; 1, ..#range(2, 5))
  mat(..#range(1, 3), ..#range(3, 5) ; ..#range(1, 4), 4) $

--- math-mat-spread-2d paged ---
#let nums = range(0, 2).map(i => (i, i+1))
$ mat(..nums, delim: "|",)
  mat(..nums; delim: "|",) $
$ mat(..nums) mat(..nums;) \
  mat(..nums;,) mat(..nums,) $

--- math-mat-spread-expected-array-error eval ---
#let nums = range(0, 2).map(i => (i, i+1))
// Error: 15-16 expected array, found content
$ mat(..nums, 0, 1) $

--- math-mat-gap paged html ---
#set math.mat(gap: 1em)
$ mat(1, 2; 3, 4) $

--- math-mat-gaps paged html ---
#set math.mat(row-gap: 1em, column-gap: 2em)
$ mat(1, 2; 3, 4) $
$ mat(column-gap: #1em, 1, 2; 3, 4)
  mat(row-gap: #2em, 1, 2; 3, 4) $

--- math-mat-augment paged ---
// Test matrix line drawing (augmentation).
#grid(
  columns: 2,
  gutter: 10pt,
  $ mat(10, 2, 3, 4; 5, 6, 7, 8; augment: #3) $,
  $ mat(10, 2, 3, 4; 5, 6, 7, 8; augment: #4) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(hline: 0)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(hline: 2)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(hline: 3)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(hline: 1, vline: 1)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(hline: -1, vline: -1)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(vline: 2, stroke: 1pt + blue)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(vline: 3, stroke: 1pt + blue)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(vline: 3, stroke: 1pt + blue)) $,
  $ mat(100, 2, 3; 4, 5, 6; 7, 8, 9; augment: #(vline: 0, stroke: 1pt + blue)) $,
  $ mat(10, 2, 3, 4; 5, 6, 7, 8; augment: #(vline: -4, stroke: 1pt + blue)) $,
)

--- math-mat-augment-set paged html ---
// Test using matrix line drawing with a set rule.
#set math.mat(augment: (hline: 2, vline: 1, stroke: 2pt + green))
$ mat(1, 0, 0, 0; 0, 1, 0, 0; 0, 0, 1, 1) $

#set math.mat(augment: 2)
$ mat(1, 0, 0, 0; 0, 1, 0, 0; 0, 0, 1, 1) $

#set math.mat(augment: none)

--- math-mat-augment-line-out-of-bounds paged html ---
// Error: 3-37 cannot draw a vertical line at offset 4 in a matrix with 3 columns
$ mat(1, 0, 0; 0, 1, 1; augment: #4) $,

--- math-mat-align paged html ---
$ mat(-1, 1, 1; 1, -1, 1; 1, 1, -1; align: #left) $
$ mat(-1, 1, 1; 1, -1, 1; 1, 1, -1; align: #center) $
$ mat(-1, 1, 1; 1, -1, 1; 1, 1, -1; align: #right) $

--- math-mat-align-explicit-alternating paged html ---
// Test alternating explicit alignment in a matrix.
$ mat(
  "a" & "a a a" & "a a";
  "a a" & "a a" & "a";
  "a a a" & "a" & "a a a";
) $

--- math-mat-align-implicit paged html ---
// Test alignment in a matrix.
$ mat(
  "a", "a a a", "a a";
  "a a", "a a", "a";
  "a a a", "a", "a a a";
) $

--- math-mat-align-explicit-left paged html ---
// Test explicit left alignment in a matrix.
$ mat(
  &"a", &"a a a", &"a a";
  &"a a", &"a a", &"a";
  &"a a a", &"a", &"a a a";
) $

--- math-mat-align-explicit-right paged html ---
// Test explicit right alignment in a matrix.
$ mat(
  "a"&, "a a a"&, "a a"&;
  "a a"&, "a a"&, "a"&;
  "a a a"&, "a"&, "a a a"&;
) $

--- math-mat-align-explicit-mixed paged html ---
// Test explicit alignment in some columns with align parameter in a matrix.
#let data = (
  ($&18&&.02$, $1$, $+1$),
  ($-&9&&.3$, $-1$, $-&21$),
  ($&&&.011$, $1$, $&0$)
)
$ #math.mat(align: left, ..data) $
$ #math.mat(align: center, ..data) $
$ #math.mat(align: right, ..data) $

--- math-mat-align-complex paged html ---
// Test #460 equations.
#let stop = {
  math.class("punctuation",$.$)
}
$ mat(&a+b,c;&d, e) $
$ mat(&a+b&,c;&d&, e) $
$ mat(&&&a+b,c;&&&d, e) $
$ mat(stop &a+b&stop,c;...stop stop&d&...stop stop, e) $

--- math-mat-align-signed-numbers paged html ---
// Test #454 equations.
$ mat(-1, 1, 1; 1, -1, 1; 1, 1, -1) $
$ mat(-1&, 1&, 1&; 1&, -1&, 1&; 1&, 1&, -1&) $
$ mat(-1&, 1&, 1&; 1, -1, 1; 1, 1, -1) $
$ mat(&-1, &1, &1; 1, -1, 1; 1, 1, -1) $

--- math-mat-bad-comma paged html ---
// Test an old issue with 2d args and named args in math.
$ mat(1, 2; 3, 4, delim: "[") $

--- issue-852-mat-type paged ---
$ mat(B, A B) $
$ mat(B, A B, dots) $
$ mat(B, A B, dots;) $
$ mat(#1, #(foo: "bar")) $

--- issue-2268-mat-augment-color paged html ---
// The augment line should be of the same color as the text
#set text(
  font: "New Computer Modern",
  lang: "en",
  fill: yellow,
)

$mat(augment: #1, M, v) arrow.r.squiggly mat(augment: #1, R, b)$

--- math-mat-delims paged html ---
$ mat(delim: #none, 1, 2; 3, 4) $

$ mat(delim: "(", 1, 2; 3, 4) $
$ mat(delim: \(, 1, 2; 3, 4) $
$ mat(delim: paren.l, 1, 2; 3, 4) $

$ mat(delim: "[", 1, 2; 3, 4) $
$ mat(delim: \[, 1, 2; 3, 4) $
$ mat(delim: bracket.l, 1, 2; 3, 4) $

$ mat(delim: "⟦", 1, 2; 3, 4) $
$ mat(delim: bracket.stroked.l, 1, 2; 3, 4) $

$ mat(delim: "{", 1, 2; 3, 4) $
$ mat(delim: \{, 1, 2; 3, 4) $
$ mat(delim: brace.l, 1, 2; 3, 4) $

$ mat(delim: "|", 1, 2; 3, 4) $
$ mat(delim: \|, 1, 2; 3, 4) $
$ mat(delim: bar.v, 1, 2; 3, 4) $

$ mat(delim: "‖", 1, 2; 3, 4) $
$ mat(delim: bar.v.double, 1, 2; 3, 4) $

$ mat(delim: "⟨", 1, 2; 3, 4) $
$ mat(delim: chevron.l, 1, 2; 3, 4) $

--- math-mat-delims-inverted paged html ---
$ mat(delim: ")", 1, 2; 3, 4) $
$ mat(delim: \), 1, 2; 3, 4) $
$ mat(delim: paren.r, 1, 2; 3, 4) $

$ mat(delim: "]", 1, 2; 3, 4) $
$ mat(delim: \], 1, 2; 3, 4) $
$ mat(delim: bracket.r, 1, 2; 3, 4) $

$ mat(delim: "⟧", 1, 2; 3, 4) $
$ mat(delim: bracket.stroked.r, 1, 2; 3, 4) $

$ mat(delim: "}", 1, 2; 3, 4) $
$ mat(delim: \}, 1, 2; 3, 4) $
$ mat(delim: brace.r, 1, 2; 3, 4) $

$ mat(delim: "⟩", 1, 2; 3, 4) $
$ mat(delim: chevron.r, 1, 2; 3, 4) $

--- math-mat-delims-pair paged html ---
$ mat(delim: #(none, "["), 1, 2; 3, 4) $
$ mat(delim: #(sym.chevron.r, sym.bracket.stroked.r), 1, 2; 3, 4) $

--- math-mat-linebreaks paged html ---
// Warning: 20-29 linebreaks are ignored in cells
// Hint: 20-29 use commas instead to separate each line
$ mat(a; b; c) mat(a \ b \ c) $

--- math-mat-linebreaks-trailing paged html ---
// Warning: 7-10 linebreaks are ignored in cells
// Hint: 7-10 use commas instead to separate each line
$ mat(a \ ) $

--- math-mat-vec-cases-unity paged html ---
// Test that matrices, vectors, and cases are all laid out the same.
$ mat(z_(n_p); a^2)
  vec(z_(n_p), a^2)
  cases(reverse: #true, delim: \(, z_(n_p), a^2)
  cases(delim: \(, z_(n_p), a^2) $

--- issue-1617-mat-align paged html ---
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$ mat(a, b; c, d) mat(x; y) $

$ x mat(a; c) + y mat(b; d)
  = mat(a x+b y; c x+d y) $

$ mat(
    -d_0, lambda_0, 0, 0, dots;
    mu_1, -d_1, lambda_1, 0, dots;
    0, mu_2, -d_2, lambda_2, dots;
    dots.v, dots.v, dots.v, dots.v, dots.down;
  )
  mat(p_0; p_1; p_2; dots.v) $

--- math-mat-delim-class paged html ---
// Test that delimiters have opening and closing math class.
$ 2mat(a, delim: bar.v) 2 $
$ 2 mat(a, delim: bar.v)2 $
//@@ file tests/suite/math_mozilla-mathml-test.typ
// Mozilla MathML Test formulas from https://mathfonts.github.io/mozilla_mathml_test.

--- mozilla-mathml-test-1 paged html ---
$ x^2 y^2 $

--- mozilla-mathml-test-2 paged html ---
$ attach(F, bl: 2, br: 3) $

--- mozilla-mathml-test-3 paged html ---
$ (x + y^2) / (k + 1) $

--- mozilla-mathml-test-4 paged html ---
$ x + y^(2 / (k + 1)) $

--- mozilla-mathml-test-5 paged html ---
$ a / frac(style: "horizontal", b, 2) $

--- mozilla-mathml-test-6 paged html ---
$
  a_0 + display(1 / (a_1 + display(1 / (a_2 + display(1 / (a_3 + display(1 / a_4)))))))
$

--- mozilla-mathml-test-7 paged html ---
$ a_0 + 1 / (a_1 + 1 / (a_2 + 1 / (a_3 + 1 / a_4))) $

--- mozilla-mathml-test-8 paged html ---
#set math.frac(style: "horizontal")
$ binom(n, k / 2) $

--- mozilla-mathml-test-9 paged html ---
$ binom(p, 2) x^2 y^(p - 2) - 1 / (1 - x) 1 / (1 - x^2) $

--- mozilla-mathml-test-10 paged html ---
$ sum_(0 <= i <= m \ 0 < j < n) P(i, j) $

--- mozilla-mathml-test-11 paged html ---
$ x^(2 y) $

--- mozilla-mathml-test-12 paged html ---
$ sum_(i = 1)^p sum_(j = 1)^q sum_(k = 1)^r a_(i j) b_(j k) c_(k i) $

--- mozilla-mathml-test-13 paged html ---
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$ sqrt(1 + sqrt(1 + sqrt(1 + sqrt(1 + sqrt(1 + sqrt(1 + sqrt(1 + x))))))) $

--- mozilla-mathml-test-14 paged html ---
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$
  (partial^2 / (partial x^2) + partial^2 / (partial y^2)) abs(phi(x + i y))^2 = 0
$

--- mozilla-mathml-test-15 paged html ---
$ 2^2^2^x $

--- mozilla-mathml-test-16 paged html ---
$ integral_1^x (dif t) / t $

--- mozilla-mathml-test-17 paged html ---
$ integral.double_D dif x dif y $

--- mozilla-mathml-test-18 paged html ---
#set math.frac(style: "horizontal")
$
  f(x) = cases(
    1/3 & "if" 0 <= x <= 1\;,
    2/3 & "if" 3 <= x <= 4\;,
    0 & "elsewhere".
  )
$

--- mozilla-mathml-test-19 paged html ---
$ overbrace(x + dots.c + x, k "times") $

--- mozilla-mathml-test-20 paged html ---
$ y_(x^2) $

--- mozilla-mathml-test-21 paged html ---
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$ sum_(p "prime") f(p) = integral_(t > 1) f(t) dif pi(t) $

--- mozilla-mathml-test-22 paged html ---
$
  { underbrace(
      overbrace(a\, ...\, a, k a's)\, overbrace(b\, ...\, b, ell b's),
      k + ell "elements"
    ) }
$

--- mozilla-mathml-test-23 paged html ---
$
  mat(
    mat(a, b; c, d), mat(e, f; g, h);
    0, mat(i, j; k, l);
  )
$

--- mozilla-mathml-test-24 paged html ---
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$
  det mat(
    delim: \|,
    c_0, c_1, c_2, dots.c, c_n;
    c_1, c_2, c_3, dots.c, c_(n + 1);
    c_2, c_3, c_4, dots.c, c_(n + 2);
    dots.v, dots.v, dots.v, , dots.v;
    c_n, c_(n + 1), c_(n + 2), dots.c, c_(2 n);
  ) > 0
$

--- mozilla-mathml-test-25 paged html ---
$ y_x_2 $

--- mozilla-mathml-test-26 paged html ---
$ x_92^31415 + pi $

--- mozilla-mathml-test-27 paged html ---
$ x^(z^d_c)_(y_b^a) $

--- mozilla-mathml-test-28 paged html ---
$ y'''_3 $

--- mozilla-mathml-test-29 paged html ---
$ lim_(n -> +oo) sqrt(2 pi n) / n! (n / e)^n = 1 $

--- mozilla-mathml-test-30 paged html ---
$
  det(A) = sum_(sigma in S_n) epsilon.alt(sigma) product_(i = 1)^n a_(i, sigma_i)
$
//@@ file tests/suite/math_multiline.typ
// Test multiline math.

--- math-align-basic paged html ---
// Test basic alignment.
$ x &= x + y \
    &= x + 2z \
    &= sum x dot 2z $

--- math-align-wider-first-column paged html ---
// Test text before first alignment point.
$ x + 1 &= a^2 + b^2 \
      y &= a + b^2 \
      z &= alpha dot beta $

--- math-align-aligned-in-source paged html ---
// Test space between inner alignment points.
$ a + b &= 2 + 3 &= 5 \
      b &= c     &= 3 $

--- math-align-cases paged html ---
// Test in case distinction.
$ f := cases(
  1 + 2 &"iff" &x,
  3     &"if"  &y,
) $

--- math-align-lines-mixed paged html ---
// Test mixing lines with and some without alignment points.
$ "abc" &= c \
   &= d + 1 \
   = x $

--- math-align-spacing paged ---
// Test how spacing is inserted around alignment points.
#set page(width: auto)
#grid(
  columns: 2,
  stroke: 1pt,
  inset: 1em,
  $
    a & + b + & c \
    a & + b   &   & e & + d \
    a & + b + & c & e & + d \
      &       & c &   & + d \
      & = 0
  $,
  $
    a & + b + & c \
    a & + b   &   & e & + d \
    a & + b + & c &   & + d \
      &       & c & e & + d \
      & = 0
  $,

  $
    a & + b + & c \
    a & + b   &   &   & + d \
    a & + b + & c & e & + d \
      &       & c & e & + d \
      & = 0
  $,
  $
    a & + b + & c \
    a & + b   &   & e & + d \
    a & + b + & c & e & + d \
      &       & c & e & + d \
      & = 0
  $,
)

--- math-align-spacing-ignorant paged ---
// Test how ignorant content affects spacing around alignment points.
#let p = place[]
$
  a + & b + & c & e &    + d \
  a + &     & c &   & #p + d
$

--- math-attach-subscript-multiline paged html ---
// Test multiline subscript.
$ sum_(n in NN \ n <= 5) n = (5(5+1))/2 = 15 $

--- math-multiline-no-trailing-linebreak paged html ---
// Test no trailing line break.
$
"abc" &= c
$
No trailing line break.

--- math-multiline-trailing-linebreak paged html ---
// Test single trailing line break.
$
"abc" &= c \
$
One trailing line break.

--- math-multiline-multiple-trailing-linebreaks paged html ---
// Test multiple trailing line breaks.
$
"abc" &= c \ \ \
$
Multiple trailing line breaks.

--- math-linebreaking-after-binop-and-rel paged ---
// Basic breaking after binop, rel
#let hrule(x) = box(line(length: x))
#hrule(45pt)$e^(pi i)+1 = 0$\
#hrule(55pt)$e^(pi i)+1 = 0$\
#hrule(70pt)$e^(pi i)+1 = 0$

--- math-linebreaking-lr paged ---
// LR groups prevent linebreaking.
#let hrule(x) = box(line(length: x))
#hrule(76pt)$a+b$\
#hrule(74pt)$(a+b)$\
#hrule(74pt)$paren.l a+b paren.r$

--- math-linebreaking-multiline paged ---
// Multiline yet inline does not linebreak
#let hrule(x) = box(line(length: x))
#hrule(80pt)$a + b \ c + d$\

--- math-linebreaking-trailing-linebreak paged ---
// A single linebreak at the end still counts as one line.
#let hrule(x) = box(line(length: x))
#hrule(60pt)$e^(pi i)+1 = 0\ $

--- math-linebreaking-in-box paged ---
// Inline, in a box, doesn't linebreak.
#let hrule(x) = box(line(length: x))
#hrule(80pt)#box($a+b$)

--- math-linebreaking-between-consecutive-relations paged ---
// A relation followed by a relation doesn't linebreak
// so essentially `a < = b` can be broken to `a` and `< = b`, `a < =` and `b`
// but never `a <` and `= b` because `< =` are consecutive relation that should
// be grouped together and no break between them.
#let hrule(x) = box(line(length: x))
#hrule(70pt)$a < = b$\
#hrule(78pt)$a < = b$

--- math-linebreaking-after-relation-without-space paged ---
// Line breaks can happen after a relation even if there is no
// explicit space.
#let hrule(x) = box(line(length: x))
#hrule(90pt)$<;$\
#hrule(95pt)$<;$\
// We don't linebreak before a closing paren, but do before an opening paren.
#hrule(90pt)$<($\
#hrule(95pt)$<($
#hrule(90pt)$<)$\
#hrule(95pt)$<)$

--- math-linebreaking-empty paged html ---
// Verify empty rows are handled ok.
$ $\
Nothing: $ $, just empty.

--- math-pagebreaking paged ---
// Test breaking of equations at page boundaries.
#set page(height: 5em)
#show math.equation: set block(breakable: true)

$ a &+ b + & c \
  a &+ b   &   && + d \
  a &+ b + & c && + d \
    &      & c && + d \
    &= 0 $

--- math-pagebreaking-numbered paged ---
// Test breaking of equations with numbering.
#set page(height: 5em)
#set math.equation(numbering: "1")
#show math.equation: set block(breakable: true)

$ a &+ b + & c \
  a &+ b   &   && + d \
  a &+ b + & c && + d \
    &      & c && + d \
    &= 0 $

--- math-pagebreaking-single-line paged ---
// Test breaking of single line equations.
#set page(height: 4em)
#show math.equation: set block(breakable: true)

Shouldn't overflow:
$ a + b $

--- math-pagebreaking-single-line-numbered paged ---
// Test breaking of single line equations with numbering.
#set page(height: 4em)
#show math.equation: set block(breakable: true)
#set math.equation(numbering: "(1)")

Shouldn't overflow:
$ a + b $

--- math-multiline-string paged html ---
$ "a\nb\n\n\n" "a\nb\n\n" "a\nb\n" "a\nb" $

--- math-multiline-string-alignment paged html ---
$ "L\n\n""L\n""L" $

--- issue-5113-pagebreaking-empty paged empty ---
// Test empty breakable equations.
#show math.equation: set block(breakable: true)
#math.equation(block: true, [])

--- issue-1948-math-text-break paged html ---
// Test text with linebreaks in math.
$ x := "a\nb\nc\nd\ne" $

--- issue-4829-math-pagebreaking-wrong-number paged ---
// Test numbering of empty regions of broken equations.
#set page(height: 5em)
#set math.equation(numbering: "1")
#show math.equation: set block(breakable: true)

#rect(height: 1.5em)

$ a + b \
  a + b $
//@@ file tests/suite/math_op.typ
// Test text operators.

--- math-op-predefined paged html ---
// Test predefined.
$ max_(1<=n<=m) n $

--- math-op-call paged html ---
// With or without parens.
$  &sin x + log_2 x \
 = &sin(x) + log_2(x) $

--- math-op-scripts-vs-limits paged html ---
// Test scripts vs limits.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
#set text(font: "New Computer Modern")
Discuss $lim_(n->oo) 1/n$ now.
$ lim_(n->infinity) 1/n = 0 $

--- math-op-custom paged html ---
// Test custom operator.
$ op("myop", limits: #false)_(x:=1) x \
  op("myop", limits: #true)_(x:=1) x $

--- math-op-symbol-baseline paged html ---
// Test baseline of a symbol wrapped in an op.
#let expect = $op(EE)$
$expect A$

--- math-op-large-stretch paged html ---
// Test that math.op's large class setting doesn't modify symbols.
$\/ class("large", \/) op(\/) integral op(integral)$
$ \/ class("large", \/) op(\/) integral op(integral) $

--- math-op-styled paged html ---
// Test styled operator.
$ bold(op("bold", limits: #true))_x y $

--- math-non-math-content paged ---
// With non-text content
$ op(#underline[ul]) a $

--- math-op-font paged html ---
// Test with different font.
#let colim = math.op(
  text(font: "IBM Plex Sans", weight: "regular", size: 0.8em)[colim],
  limits: true,
)
$ colim_(x -> 0) inline(colim_(x -> 0)) $

--- math-op-set-font paged html ---
// Test setting font.
#show math.equation: set text(weight: "regular")
#let lig = math.op("fi")
#let test = $sech(x) mod_(x -> oo) lig_1(X)$
#test
#show math.op: set text(font: "New Computer Modern")
#test

--- math-op-stretch-relative paged html ---
// Test that math.op's large class setting doesn't affect relative stretch
// resolution.
$ op(stretch(|, size: #300%)) stretch(|, size: #300%) $
//@@ file tests/suite/math_primes.typ
--- math-primes paged html ---
// Test dedicated syntax for primes
$a'$, $a'''_b$, $'$, $'''''''$

--- math-primes-spaces paged html ---
// Test spaces between
$a' ' '$, $' ' '$, $a' '/b$

--- math-primes-complex paged html ---
// Test complex prime combinations
$a'_b^c$, $a_b'^c$, $a_b^c'$, $a_b'^c'^d'$

$(a'_b')^(c'_d')$, $a'/b'$, $a_b'/c_d'$

$∫'$, $∑'$, $a'^2^2$, $a'_2_2$

$f_n'^a'$, $f^a'_n'$

$ ∑'_S' $

--- math-primes-attach paged html ---
// Test attaching primes only
$a' = a^', a_', a_'''^''^'$

--- math-primes-factorial paged html ---
// Test edge cases with factorials and primes
$
  n'!' quad n' !' quad a_n'!'^b \
  n!'! quad n! '! quad a_n!'!^b \
$

--- math-primes-scripts paged html ---
// Test primes always attaching as scripts
$ x' $
$ x^' $
$ attach(x, t: ') $
$ <' $
$ attach(<, br: ') $
$ op(<, limits: #true)' $
$ limits(<)' $

--- math-primes-limits paged html ---
// Test forcefully attaching primes as limits
$ attach(<, t: ') $
$ <^' $
$ attach(<, b: ') $
$ <_' $

$ limits(x)^' $
$ attach(limits(x), t: ') $

--- math-primes-merge-top paged html ---
// Test prime attachment merging with the top field.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$
               attach(a, tr: ', t: b)
  &quad attach(attach(a, tr: '), t: b)
  &quad attach(attach(a, t: b), tr: ')
  &quad attach(attach(attach(a, tl: '), t: b), tr: ')
  &quad attach(attach(attach(a, tr: '), t: b), tr: ')
  \
  // When the base has limits, top prime merging is invariant of t/tr order.
               attach(product, tr: ', t: b)
  &quad attach(attach(product, tr: '), t: b)
  &quad attach(attach(product, t: b), tr: ')
  &quad attach(attach(attach(product, tl: '), t: b), tr: ')
  &quad attach(attach(attach(product, tr: '), t: b), tr: ')
$

--- math-primes-merge-top-nested paged html ---
// Test prime-top merging with an additional inner attachment.
// The first row should attach as prime-2, the second row as 2-prime.
$
                     attach(a, b: 1, tr: ', t: 2)
  quad        attach(attach(b, b: 1, tr: '), t: 2)
  quad        attach(attach(c, b: 1), tr: ', t: 2)
  quad attach(attach(attach(d, b: 1), tr: '), t: 2)
  \
              attach(attach(e, b: 1, t: 2), tr: ')
  quad attach(attach(attach(f, b: 1), t: 2), tr: ')
$

--- math-primes-merge-inner-prime paged html ---
// Don't join t and tr when there is an outer tr prime.
$
  attach(attach(a, tr: '), t: b, tr: c)
  quad
  attach(attach(a, tr: ', t: b), tr: ')
  quad
  attach(attach(a, tr: c, t: b), tr: ')
$

--- math-primes-after-code-expr paged html ---
// Test prime symbols after code mode.
#let g = $f$
#let gg = $f$

$
  #(g)' #g' #g ' \
  #g''''''''''''''''' \
  gg'
$

--- math-primes-with-superscript paged html ---
// Test prime symbols don't raise the superscript position
$
  sqrt(f)/f
  sqrt(f^2)/f^2
  sqrt(f'^2)/f'^2
  sqrt(f''_n^2)/f''^2_n
$

--- math-primes-show-rules paged html ---
#show math.primes: set text(red)
$ x' x'' x''' x'''' x''''' x'''''' $
#show math.prime: set text(blue)
$ x' x'' x''' x'''' x''''' x'''''' $
//@@ file tests/suite/math_root.typ
// Test roots.

--- math-root-basic paged html ---
// Test root with more than one character.
$A = sqrt(x + y) = c$

--- math-root-radical-attachment paged html ---
// Test root size with radicals containing attachments.
$ sqrt(a) quad
  sqrt(f) quad
  sqrt(q) quad
  sqrt(a^2) \
  sqrt(n_0) quad
  sqrt(b^()) quad
  sqrt(b^2) quad
  sqrt(q_1^2) $

--- math-root-precomposed paged html ---
// Test precomposed vs constructed roots.
// 3 and 4 are precomposed.
$sqrt(x)$
$root(2, x)$
$root(3, x)$
$root(4, x)$
$root(5, x)$

--- math-root-large-body paged html ---
// Test large bodies
$ sqrt([|x|]^2 + [|y|]^2) < [|z|] $
$ v = sqrt((1/2) / (4/5))
   = root(3, (1/2/3) / (4/5/6))
   = root(4, ((1/2) / (3/4)) / ((1/2) / (3/4))) $
$ v = sqrt(a +\ b) $

--- math-root-large-index paged html ---
// Test large index.
$ root(2, x) quad
  root(3/(2/1), x) quad
  root(1/11, x) quad
  root(1/2/3, 1) $

--- math-root-syntax paged html ---
// Test shorthand.
$ √2^3 = sqrt(2^3) $
$ √(x+y) quad ∛x quad ∜x $
$ (√2+3) = (sqrt(2)+3) $

--- math-root-syntax-prec paged html ---
// Precedence of root syntax with other math operators.
$ √a/b ∛a_b ∜f' √n! \
  √a b^c  √a (b)^c  √a(b)^c $

--- math-root-frame-size-index paged html ---
// Test size of final frame when there is an index.
$ a root(, 3)         & a root(., 3) \
  a sqrt(3)           & a root(2, 3) \
  a root(#h(-1em), 3) & a root(123, 3) $

--- math-root-line-fill-stroke paged html ---
// Test that the horizontal stroke is also decorated like text glyphs
#text(size: 20pt, fill: yellow, stroke: red + .5pt)[$sqrt(Delta)$]
#text(size: 25pt, stroke: red)[$root(3, Delta)$]

--- math-root-index-cramped paged html ---
// Test whether the index has cramped styles.
$ root(x^2^2, 1) $
//@@ file tests/suite/math_size.typ
--- math-size paged html ---
// Test forcing math size
$a/b, display(a/b), display(a)/display(b), inline(a/b), script(a/b), sscript(a/b) \
 mono(script(a/b)), script(mono(a/b))\
 script(a^b, cramped: #true), script(a^b, cramped: #false)$

--- issue-3658-math-size paged ---
$ #rect[$1/2$] $
$#rect[$1/2$]$
//@@ file tests/suite/math_spacing.typ
// Test spacing in math formulas.

--- math-spacing-basic paged html ---
// Test spacing cases.
$ä, +, c, (, )$ \
$=), (+), {times}$ \
$⟧<⟦, abs(-), [=$ \
$a=b, a==b$ \
$-a, +a$ \
$a not b$ \
$a+b, a*b$ \
$sum x, sum(x)$ \
$sum product x$ \
$f(x), zeta(x), "frac"(x)$ \
$a+dots.c+b$
$f(x) sin(y)$

--- math-spacing-kept-spaces paged html ---
// Test ignored vs non-ignored spaces.
$f (x), f(x)$ \
$[a|b], [a | b]$ \
$a"is"b, a "is" b$ \
$A"B"C, A "B" C$

--- math-spacing-predefined paged html ---
// Test predefined spacings.
$a thin b, a med b, a thick b, a quad b$ \
$a = thin b$ \
$a - b equiv c quad (mod 2)$

--- math-spacing-set-comprehension paged html ---
// Test spacing for set comprehension.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}
$ { x in RR | x "is natural" and x < 10 } $

--- math-spacing-decorated paged ---
// Test spacing for operators with decorations and modifiers on them
#set page(width: auto)
$a equiv b + c - d => e log 5 op("ln") 6$ \
$a cancel(equiv) b overline(+) c arrow(-) d hat(=>) e cancel(log) 5 dot(op("ln")) 6$ \
$a overbrace(equiv) b underline(+) c grave(-) d underbracket(=>) e circle(log) 5 caron(op("ln")) 6$ \
\
$a attach(equiv, tl: a, tr: b) b attach(limits(+), t: a, b: b) c tilde(-) d breve(=>) e attach(limits(log), t: a, b: b) 5 attach(op("ln"), tr: a, bl: b) 6$

--- math-spacing-weak paged html ---
// Test weak spacing
$integral f(x) dif x$,
// Not weak
$integral f(x) thin dif x$,
// Both are weak, collide
$integral f(x) #h(0.166em, weak: true)dif x$

--- math-spacing-weak-script paged html ---
// Test weak spacing in script size
$a #h(0.9em, weak: true) sscript(#h(1em, weak: true)) b$ \
$a #h(0.9em) b$

--- math-spacing-script paged html ---
// Test spacing in script size
$x^(a #h(1em) b) + x^x^(a #h(1em) b) + sscript(a #h(1em) b)$

--- math-spacing-ignorant paged ---
// Test spacing with ignorant elements
$#metadata(none) "text"$ \
$#place(dx: 5em)[Placed] "text"$ \
// Operator spacing
$#counter("test").update(3) + b$ \
$#place(dx: 5em)[a] + b$
// Validate that ignorant elements are layouted
#context test(counter("test").get(), (3,))

--- issue-1052-math-number-spacing paged html ---
// Test spacing after numbers in math.
$
10degree \
10 degree \
10.1degree \
10.1 degree
$
//@@ file tests/suite/math_stretch.typ
// Test math stretch.

--- math-stretch-basic paged html ---
// Test basic stretch.
$ P -> Q stretch(->, size: #200%) R \
  R stretch(->) S stretch(->, size: #50%)^"epimorphism" T $

--- math-stretch-complex paged html ---
// Test complex stretch.
$ H stretch(=)^"define" U + p V \
  x stretch(harpoons.ltrb, size: #3em) y
    stretch(\[, size: #150%) z \
  f : X stretch(arrow.hook, size: #150%)_"injective" Y \
  V stretch(->, size: #(100% + 1.5em))^("surjection") ZZ $

--- math-stretch-horizontal-attach paged html ---
// Test horizontal stretch interactions with attachments.
#show: it => context {
  set page(width: auto) if target() == "paged"
  it
}

$stretch(stretch(=, size: #4em))_A$
$stretch(arrow.hook, size: #5em)^"injective map"$
$stretch(arrow.hook, size: #200%)^"injective map"$

$ P = Q
    stretch(=)^(k = 0)_(forall i) R
    stretch(=, size: #150%)^(k = 0)_(forall i) S
    stretch(=, size: #2mm)^(k = 0)_(forall i) T \
  U stretch(equiv)^(forall i)_"Chern-Weil" V
    stretch(equiv, size: #(120% + 2mm))^(forall i)_"Chern-Weil" W $

--- math-stretch-horizontal-attach-constants paged html ---
#show math.equation: set text(font: "Asana Math")
$ a stretch(->)^a b ->^a c $
$ a stretch(->)_p b ->_p c $

--- math-stretch-vertical-attach paged html ---
// Test vertical stretch interactions with attachments.
$arrow.t$
$stretch(arrow.t)^"map"$
$stretch(arrow.t, size: #2em)^"map"$
$stretch(arrow.t, size: #200%)^"map"$

--- math-stretch-vertical-large-class paged html ---
// Test vertical stretch of large math class characters that are stretched in
// display size automatically.
$integral
 stretch(integral, size: #3em)
 stretch(integral, size: #0em)
 stretch(integral, size: #50%)
 stretch(integral, size: #200%)$
$ integral
  stretch(integral, size: #3em)
  stretch(integral, size: #0em)
  stretch(integral, size: #50%)
  stretch(integral, size: #200%) $

--- math-stretch-nested-vertical-attach paged html ---
// Test nested vertical stretch interactions with attachments.
$ stretch(stretch(\[, size: #4em))
  stretch(stretch(\[, size: #4em), size: #0em)
  stretch(stretch(\[, size: #200%))
  stretch(stretch(\[, size: #200%), size: #50%)
  =
  stretch(stretch(\[, size: #4em))_A
  stretch(stretch(\[, size: #4em), size: #0em)_A
  stretch(stretch(\[, size: #200%))_A
  stretch(stretch(\[, size: #200%), size: #50%)_A $

--- math-stretch-nested-horizontal-attach paged html ---
// Test nested horizontal stretch interactions with attachments.
$ stretch(stretch(->, size: #4em)) >> stretch(stretch(->, size: #4em))_A \
  stretch(stretch(->, size: #4em), size: #0em) = stretch(stretch(->, size: #4em), size: #0em)_A \
  stretch(stretch(->, size: #500%)) >>> stretch(stretch(->, size: #500%))_A \
  stretch(stretch(->, size: #500%), size: #50%) > stretch(stretch(->, size: #500%), size: #50%)_A \
  stretch(stretch(->, size: #4em), size: #50%) > stretch(stretch(->, size: #4em), size: #50%)_"blah" $

--- math-stretch-lr-nested-vertical paged html ---
// Test stretch and lr nested vertical interactions.
$ stretch(lr(arrow.t, size: #3em))
  stretch(lr(arrow.t, size: #3em), size: #0em)
  stretch(lr(arrow.t, size: #3em), size: #50%)
  stretch(lr(arrow.t, size: #3em), size: #200%),
  lr(stretch(arrow.t, size: #3em))
  lr(stretch(arrow.t, size: #3em), size: #0em)
  lr(stretch(arrow.t, size: #3em), size: #50%)
  lr(stretch(arrow.t, size: #3em), size: #200%),
  stretch(lr(arrow.t), size: #3em)
  stretch(lr(arrow.t, size: #0em), size: #3em)
  stretch(lr(arrow.t, size: #50%), size: #3em)
  stretch(lr(arrow.t, size: #200%), size: #3em),
  lr(stretch(arrow.t), size: #3em)
  lr(stretch(arrow.t, size: #0em), size: #3em)
  lr(stretch(arrow.t, size: #50%), size: #3em)
  lr(stretch(arrow.t, size: #200%), size: #3em) $

--- math-stretch-lr-nested-vertical-attach paged html ---
// Test stretch and lr nested vertical interactions with attachments.
$ stretch(lr(arrow.t, size: #3em))^A
  stretch(lr(arrow.t, size: #3em), size: #0em)^A
  stretch(lr(arrow.t, size: #3em), size: #50%)^A
  stretch(lr(arrow.t, size: #3em), size: #200%)^A,
  lr(stretch(arrow.t, size: #3em))^A
  lr(stretch(arrow.t, size: #3em), size: #0em)^A
  lr(stretch(arrow.t, size: #3em), size: #50%)^A
  lr(stretch(arrow.t, size: #3em), size: #200%)^A,
  stretch(lr(arrow.t), size: #3em)^A
  stretch(lr(arrow.t, size: #0em), size: #3em)^A
  stretch(lr(arrow.t, size: #50%), size: #3em)^A
  stretch(lr(arrow.t, size: #200%), size: #3em)^A,
  lr(stretch(arrow.t), size: #3em)^A
  lr(stretch(arrow.t, size: #0em), size: #3em)^A
  lr(stretch(arrow.t, size: #50%), size: #3em)^A
  lr(stretch(arrow.t, size: #200%), size: #3em)^A $

--- math-stretch-lr-nested-horizontal paged html ---
// Test stretch and lr nested horizontal interactions.
$ stretch(lr(=, size: #2em))
  stretch(lr(=, size: #2em), size: #0em)
  stretch(lr(=, size: #2em), size: #50%)
  stretch(lr(=, size: #2em), size: #200%) \
  lr(stretch(=, size: #2em))
  lr(stretch(=, size: #2em), size: #0em)
  lr(stretch(=, size: #2em), size: #50%)
  lr(stretch(=, size: #2em), size: #200%) \
  stretch(lr(=), size: #2em)
  stretch(lr(=, size: #0em), size: #2em)
  stretch(lr(=, size: #50%), size: #2em)
  stretch(lr(=, size: #200%), size: #2em) \
  lr(stretch(=), size: #2em)
  lr(stretch(=, size: #0em), size: #2em)
  lr(stretch(=, size: #50%), size: #2em)
  lr(stretch(=, size: #200%), size: #2em) $

--- math-stretch-lr-nested-horizontal-attach paged html ---
// Test stretch and lr nested horizontal interactions with attachments.
$ stretch(lr(=, size: #2em))_A
  stretch(lr(=, size: #2em), size: #0em)_A
  stretch(lr(=, size: #2em), size: #50%)_A
  stretch(lr(=, size: #2em), size: #200%)_A \
  lr(stretch(=, size: #2em))_A
  lr(stretch(=, size: #2em), size: #0em)_A
  lr(stretch(=, size: #2em), size: #50%)_A
  lr(stretch(=, size: #2em), size: #200%)_A \
  stretch(lr(=), size: #2em)_A
  stretch(lr(=, size: #0em), size: #2em)_A
  stretch(lr(=, size: #50%), size: #2em)_A
  stretch(lr(=, size: #200%), size: #2em)_A \
  lr(stretch(=), size: #2em)_A
  lr(stretch(=, size: #0em), size: #2em)_A
  lr(stretch(=, size: #50%), size: #2em)_A
  lr(stretch(=, size: #200%), size: #2em)_A $

--- math-stretch-vertical-scripts paged html ---
// Test vertical stretch interactions with script attachments.
#let big = $stretch(|, size: #4em)$
$ big_0^1 stretch(|, size: #1.5em)_0^1
  stretch(big, size: #1em)_0^1 |_0^1 $

--- math-stretch-horizontal paged html ---
// Test stretching along horizontal axis.
#let ext(sym) = math.stretch(sym, size: 2em)
$ ext(arrow.r) quad ext(arrow.l.double.bar) \
  ext(harpoon.rb) quad ext(harpoons.ltrb) \
  ext(paren.t) quad ext(shell.b) \
  ext(eq) quad ext(equiv) $

--- math-stretch-vertical paged html ---
// Test stretching along vertical axis.
#let ext(sym) = math.stretch(sym, size: 2em)
$ ext(bar.v) quad ext(bar.v.double) quad
  ext(chevron.l) quad ext(chevron.r) quad
  ext(paren.l) quad ext(paren.r) \
  ext(bracket.l.stroked) quad ext(bracket.r.stroked) quad
  ext(brace.l) quad ext(brace.r) quad
  ext(bracket.l) quad ext(bracket.r) $

--- math-stretch-shorthand paged html ---
// Test stretch when base is given with shorthand.
$stretch(||, size: #2em)$
$stretch(\(, size: #2em)$
$stretch(⟧, size: #2em)$
$stretch(|, size: #2em)$
$stretch(->, size: #2em)$
$stretch(↣, size: #2em)$

--- math-stretch-nested paged html ---
// Test nested stretch calls.
$ stretch(=, size: #2em) \
  stretch(stretch(=, size: #4em), size: #50%) $

#let base = math.stretch($=$, size: 4em)
$ stretch(base, size: #50%) $

#let base = $stretch(=, size: #4em) $
$ stretch(base, size: #50%) $

--- math-stretch-attach-nested-equation paged html ---
// Test stretching with attachments when nested in an equation.
#let body = $stretch(=)$
$ body^"text" $

#{
  let body = $stretch(=)$
  for i in range(24) {
    body = $body$
  }
  $body^"long text"$
}

--- math-stretch-min-overlap-exceeds-max paged ---
// Test that glyph assembly doesn't end up with negative lengths if the max
// overlap calculated is less than the minConnectorOverlap.
#show math.equation: set text(font: "STIX Two Math")
// Warning: glyph has assembly parts with overlap less than minConnectorOverlap
// Hint: its rendering may appear broken - this is probably a font bug
// Hint: please file an issue at https://github.com/typst/typst/issues
$ stretch(->)^"Gauss-Jordan Elimination" $

--- math-stretch-explicit-mixed paged html ---
$ stretch(size: #200%, \/) quad
  \/ quad
  class("large", \/) quad
  class("large", stretch(size: #200%, \/)) quad
  stretch(size: #200%, class("large", \/)) $

--- math-stretch-fallback paged ---
// Test that stretching works in fonts with ssty variants.
#show math.equation: set text(font: "STIX Two Math")
$ lr(a / b|)_sqrt(c / d) $
$ integral_(-oo)^oo e^(-(m omega)/(2 planck) (x^2 + (2 i p)/(m omega) x)) dif x $

--- math-stretch-fallback-explicit paged ---
// Test stretching fallback when features are applied.
#show math.equation: set text(font: "STIX Two Math")
$ sscript(sqrt(a / b) quad (c / d)) $

--- math-no-stretch-fallback paged ---
// Test that stretching fallback doesn't happen if the original size suffices.
#show math.equation: set text(font: "STIX Two Math")
$ script(sqrt(x) (a)) $
//@@ file tests/suite/math_style.typ
// Test text styling in math.

--- math-style-italic-default paged html ---
// Test italic defaults.
$a, A, delta, ϵ, partial, Delta, ϴ$

--- math-style paged html ---
// Test forcing a specific style.
$A, italic(A), upright(A), bold(A), bold(upright(A)), \
 serif(A), sans(A), cal(A), frak(A), mono(A), bb(A), \
 italic(partial), upright(partial), \
 bb("hello") + bold(cal("world")), \
 mono("SQRT")(x) wreath mono(123 + 456)$

--- math-style-fallback paged html ---
// Test how math styles fallback.
$upright(frak(bold(alpha))) = upright(bold(alpha)) \
bold(mono(ϝ)) = bold(ϝ) \
sans(Theta) = bold(sans(Theta)) \
bold(upright(planck)) != planck \
bb(e) != italic(bb(e)) \
serif(sans(A)) != serif(A)$

--- math-style-dotless paged html ---
// Test styling dotless i and j.
$ dotless.i dotless.j,
  upright(dotless.i) upright(dotless.j),
  sans(dotless.i) sans(dotless.j),
  bold(dotless.i) bold(dotless.j),
  bb(dotless.i) bb(dotless.j),
  cal(dotless.i) cal(dotless.j),
  frak(dotless.i) frak(dotless.j),
  mono(dotless.i) mono(dotless.j),
  bold(frak(dotless.i)) upright(sans(dotless.j)),
  italic(bb(dotless.i)) frak(sans(dotless.j)) $

--- math-style-font-no-dotless paged html ---
// Test dotless i and j codepoints work without the dtls feature. In
// particular, make sure that the variation selector is not present with scr
// and cal.
#show math.equation: set text(font: "Libertinus Math")
$ dotless.i dotless.j, upright(dotless.i dotless.j),
  scr(dotless.i dotless.j), cal(dotless.i dotless.j),
  frak(dotless.i dotless.j), mono(dotless.i dotless.j),
  upright(bold(dotless.i dotless.j)), bold(upright(sans(dotless.i dotless.j))) $

--- math-style-exceptions paged html ---
// Test a few style exceptions.
$h, bb(N), cal(R), Theta, italic(Theta), sans(Theta), sans(italic(Theta)) \
 bb(d), bb(italic(d)), italic(bb(d)), bb(e), bb(italic(e)), italic(bb(e)) \
 bb(i), bb(italic(i)), italic(bb(i)), bb(j), bb(italic(j)), italic(bb(j)) \
 bb(D), bb(italic(D)), italic(bb(D))$

--- math-style-greek-exceptions paged html ---
// Test a few greek exceptions.
$bb(Gamma) , bb(gamma), bb(Pi), bb(pi), bb(sum)$

--- math-style-hebrew-exceptions paged html ---
// Test hebrew exceptions.
$aleph, beth, gimel, daleth$ \
$upright(aleph), upright(beth), upright(gimel), upright(daleth)$

--- math-style-script paged html ---
// Test variation selectors for scr and cal.
$cal(A) scr(A) bold(cal(O)) scr(bold(O))$

#show math.equation: set text(font: "Noto Sans Math")
$scr(E) cal(E) bold(scr(Y)) cal(bold(Y))$

--- math-style-cluster paged html ---
// Test styling a grapheme cluster.
#let cluster = symbol("U\u{fe00}")
$cluster bb(cluster) bold(sans(upright(cluster))) scr(cluster) bold(cal(cluster))$

--- issue-3650-italic-equation paged ---
_abc $sin(x) "abc"$_ \
$italic(sin(x) "abc" #box[abc])$ \
*abc $sin(x) "abc"$* \
$bold(sin(x) "abc" #box[abc])$ \

--- issue-8520-script-stylistic-set paged ---
$ cal(B) cal(L) cal(P) cal(R) $
#show math.equation: set text(weight: 400)
$ cal(B) cal(L) cal(P) cal(R) $
//@@ file tests/suite/math_symbols.typ
// Test math symbol edge cases.

--- math-symbol-basic eval ---
#let sym = symbol("s", ("basic", "s"))
#test($sym.basic$, $s$)

--- math-symbol-underscore eval ---
#let sym = symbol("s", ("test_underscore", "s"))
// Error: 6-10 unknown symbol modifier
$sym.test_underscore$

--- math-symbol-dash eval ---
#let sym = symbol("s", ("test-dash", "s"))
// Error: 6-10 unknown symbol modifier
$sym.test-dash$

--- math-symbol-double eval ---
#let sym = symbol("s", ("test.basic", "s"))
#test($sym.test.basic$, $s$)

--- math-symbol-double-underscore eval ---
#let sym = symbol("s", ("one.test_underscore", "s"))
// Error: 10-14 unknown symbol modifier
$sym.one.test_underscore$

--- math-symbol-double-dash eval ---
#let sym = symbol("s", ("one.test-dash", "s"))
// Error: 10-14 unknown symbol modifier
$sym.one.test-dash$
//@@ file tests/suite/math_syntax.typ
// Test math syntax.

--- math-unicode paged html ---
// Test Unicode math.
$ ∑_(i=0)^ℕ a ∘ b = \u{2211}_(i=0)^NN a compose b $

--- math-shorthands paged ---
// Test a few shorthands.
$ underline(f' : NN -> RR) \
  n |-> cases(
    [|1|] &"if" n >>> 10,
    2 * 3 &"if" n != 5,
    1 - 0 thick &...,
  ) $

--- math-shorthands-noncontinuable paged html ---
// Test that shorthands are not continuable.
$ x >=(y) / z \
  x >= (y) / z $

--- math-common-symbols paged html ---
// Test common symbols.
$ dot \ dots \ ast \ tilde \ star $

--- issue-2044-invalid-parsed-ident paged html ---
// In this bug, the dot at the end was causing the right parenthesis to be
// parsed as an identifier instead of the closing right parenthesis.
$floor(phi.alt.)$
$floor(phi.alt. )$

--- math-ident-invalid-grapheme-at-end-of-ident eval ---
// The final grapheme in `ident⃝` starts with a letter, but ends with U+20DD
// COMBINING ENCLOSING CIRCLE, which is not part of `xid_continue`.
//
// Instead of including this non-ident character in the identifier, we split the
// tokens in the middle of the grapheme. This matches the behavior of code mode,
// but is mainly for implementation simplicity.
#let ident = "A"
#let eqn = $ ident⃝ $
#test(repr(eqn.body), "sequence([A], [⃝])")

--- issue-4828-math-number-multi-char paged html ---
// Numbers should parse the same regardless of number of characters.
$1/2(x)$ vs. $1/10(x)$

--- math-unclosed eval ---
// Error: 1-2 unclosed delimiter
$a
//@@ file tests/suite/math_text.typ
// Test that setting font features in math.equation has an effect.

--- math-font-fallback paged html ---
// Test font fallback.
$ よ and 🏳️‍🌈 $

--- math-text-color paged html ---
// Test text properties.
$text(#red, "time"^2) + sqrt("place")$

--- math-text-single-grapheme-cluster paged html ---
// Test that single graph clusters are considered a single character in math.
$ 𝒟 𝒟︀ 𝒟︁ $
#show math.equation: set text(font: "Noto Sans Math")
$ 𝒟 𝒟︀ 𝒟︁ $

--- math-text-grapheme-cluster-fallback paged ---
// Test fallback with grapheme clusters.
#let bird = symbol("🐦‍⬛")
#bird or 🐦‍⬛
$bird "or" 🐦‍⬛$

#set text(font: "Noto Color Emoji")
#show math.equation: set text(font: "Noto Color Emoji")
#bird or 🐦‍⬛
// Warning: 1-16 font family is not designed for math: Noto Color Emoji
// Hint: 1-16 rendering may be poor
$bird "or" 🐦‍⬛$

--- math-text-styled-num paged html ---
// Test that styled numbers are treated the same as unstyled ones.
$ a"123.4"b quad a "123.4" b $
#show text: math.bold
$ a"123.4"b quad a "123.4" b $

--- math-font-features paged html ---
$ nothing $
$ "hi ∅ hey" $
$ sum_(i in NN) 1 + i $
#show math.equation: set text(features: ("cv02",), fallback: false)
$ nothing $
$ "hi ∅ hey" $
$ sum_(i in NN) 1 + i $

--- math-font-features-switch paged html ---
#let scr(it) = text(stylistic-set: 1, $cal(it)$)
$cal(P)_i != scr(P)_i$, $cal(bold(I))_l != bold(scr(I))_l$
$ product.co_(B in scr(B))^(B in scr(bold(B))) cal(B)(X) $

--- math-font-covers paged html ---
#show math.equation: set text(
  font: (
    // Ignore that this regex actually misses some of the script glyphs...
    (name: "XITS Math", covers: regex("[\u{1D49C}-\u{1D503}]")),
    "New Computer Modern Math"
  ),
  stylistic-set: 1,
)
$ cal(P)_i (X) * cal(C)_1 $

--- math-font-warning paged ---
#show math.equation: set text(font: "Libertinus Serif")
// Warning: 1-14 font family is not designed for math: Libertinus Serif
// Hint: 1-14 rendering may be poor
$ x + y = z $

--- math-font-error paged ---
// Warning: 37-51 unknown font family: cambria math
#show math.equation: set text(font: "Cambria Math", fallback: false)
// Error: 1-39 no font could be found
$ brace.stroked.l -1 brace.stroked.r $

--- math-font-fallback-class paged html ---
// Test that math class is preserved even when the result is a tofu.
#show math.equation: set text(font: "Garamond-Math", fallback: false)
$ brace.stroked.l -1 brace.stroked.r $
$ lr(brace.stroked.l -1 brace.stroked.r) $

--- math-optical-size-nested-scripts paged html ---
// Test transition from script to scriptscript.
#[
#set text(size:20pt)
$  e^(e^(e^(e))) $
]
A large number: $e^(e^(e^(e)))$.

--- math-optical-size-primes paged html ---
//  Test prime/double prime via scriptsize
#let prime = [ \u{2032} ]
#let dprime = [ \u{2033} ]
#let tprime = [ \u{2034} ]
$ y^dprime-2y^prime + y = 0 $
$y^dprime-2y^prime + y = 0$
$ y^tprime_3 + g^(prime 2) $

--- math-optical-size-prime-large-operator paged html ---
// Test prime superscript on large symbol
$ scripts(sum_(k in NN))^prime 1/k^2 $
$sum_(k in NN)^prime 1/k^2$

--- math-optical-size-frac-script-script paged html ---
// Test script-script in a fraction.
$ 1/(x^A) $
#[#set text(size:18pt); $1/(x^A)$] vs. #[#set text(size:14pt); $x^A$]

--- math-par paged ---
// Ensure that math does not produce paragraphs.
#show par: highlight
$ a + "bc" + #[c] + #box[d] + #block[e] $

--- issue-6090-math-overhang paged html ---
$ f(t) = cases(
    1 quad & "if" 0 < t < 1\,,
    0 quad & "otherwise"
) $
$ f(t) = cases(
    1 quad & "if" 0 < t < 1\,,
    0 quad & "otherwise.",
) $
$ f(t) = cases(
    1 quad & "if" 0 < t < 1\,,
    0 quad & "otherwise,",
) $
$ f(t) = cases(
    1 quad & "if" 0 < t < 1\,,
    0 quad & "otherwise!",
) $

--- issue-8261-string-as-empty paged html ---
// Testing that empty strings produce no element in MathML
$ "" " " $

--- issue-8261-string-as-ellipsis paged html ---
// Testing that ellipses produce `<mtext>` in MathML, not `<mn>`
$ "..." $

--- issue-8261-string-as-non-numeric paged html ---
// Testing that various "bogus" strings produce `<mtext>` in MathML, not `<mn>`
$ "1..1" ".1.1" "1.1." "1..1..1" "1.1.1" ".." $
//@@ file tests/suite/math_underover.typ
// Test under/over things.

--- math-underover-brace paged html ---
// Test braces.
$ x = underbrace(
  1 + 2 + ... + 5,
  underbrace("numbers", x + y)
) $

--- math-underover-line-bracket paged ---
// Test lines and brackets.
$ x = overbracket(
  overline(underline(x + y)),
  1 + 2 + ... + 5,
) $

--- math-underover-brackets paged html ---
// Test brackets.
$ underbracket([1, 2/3], "relevant stuff")
          arrow.l.r.double.long
  overbracket([4/5,6], "irrelevant stuff") $

--- math-underover-parens paged html ---
// Test parentheses.
$ overparen(
  underparen(x + y, "long comment"),
  1 + 2 + ... + 5
) $

--- math-underover-shells paged html ---
// Test tortoise shell brackets.
$ undershell(
  1 + overshell(2 + ..., x + y),
  "all stuff"
) $

--- math-underover-line-subscript paged ---
// Test effect of lines on subscripts.
$A_2 != overline(A)_2 != underline(A)_2 != underline(overline(A))_2 \
 V_y != overline(V)_y != underline(V)_y != underline(overline(V))_y \
 W_l != overline(W)_l != underline(W)_l != underline(overline(W))_l$

--- math-underover-line-superscript paged ---
// Test effect of lines on superscripts.
$J^b != overline(J)^b != underline(J)^b != underline(overline(J))^b \
 K^3 != overline(K)^3 != underline(K)^3 != underline(overline(K))^3 \
 T^i != overline(T)^i != underline(T)^i != underline(overline(T))^i$

--- math-underover-multiline-annotation paged html ---
// Test that multiline annotations do not change the baseline.
$ S = overbrace(beta (alpha) S I, "one line")
    - overbrace(mu (N), "two" \  "line") $
$ S = underbrace(beta (alpha) S I, "one line")
    - underbrace(mu (N), "two" \  "line") $

--- math-underover-multiline-annotation-align paged html ---
$ x = underbrace(
  "complexity",
  // The alignment points are not shared between body and annotation.
  underbrace(&1+2 \ 3+&4, a b+&c \ d+&e)
) $

--- math-underover-line-fill-stroke paged ---
// Test that the horizontal stroke is also decorated like text glyphs
#text(size: 20pt, fill: yellow, stroke: red + .5pt)[$underline(Delta).overline(Delta)$]
#text(size: 25pt, stroke: red)[$underline(Delta).overline(Delta)$]
//@@ file tests/suite/math_vec.typ
// Test vectors.

--- math-vec-gap paged html ---
#set math.vec(gap: 1em)
$ vec(1, 2) $

--- math-vec-align paged html ---
$ vec(-1, 1, -1, align: #left)
  vec(-1, 1, -1, align: #center)
  vec(-1, 1, -1, align: #right) $

--- math-vec-align-explicit-alternating paged html ---
// Test alternating alignment in a vector.
$ vec(
  "a" & "a a a" & "a a",
  "a a" & "a a" & "a",
  "a a a" & "a" & "a a a",
) $

--- math-vec-wide paged html ---
// Test wide cell.
$ v = vec(1, 2+3, 4) $

--- math-vec-delim-set paged html ---
// Test alternative delimiter.
#set math.vec(delim: "[")
$ vec(1, 2) $

--- math-vec-delim-empty-string eval ---
// Error: 22-24 expected exactly one character
#set math.vec(delim: "")

--- math-vec-delim-not-single-char eval ---
// Error: 22-39 expected exactly one character
#set math.vec(delim: "not a delimiter")

--- math-vec-delim-invalid-char eval ---
// Error: 22-25 invalid delimiter: "%"
#set math.vec(delim: "%")

--- math-vec-delim-invalid-symbol eval ---
// Error: 22-33 invalid delimiter: "%"
#set math.vec(delim: sym.percent)

--- math-vec-delim-invalid-opening eval ---
// Error: 22-33 invalid delimiter: "%"
#set math.vec(delim: ("%", none))

--- math-vec-delim-invalid-closing eval ---
// Error: 22-33 invalid delimiter: "%"
#set math.vec(delim: (none, "%"))

--- math-vec-linebreaks paged html ---
// Warning: 20-29 linebreaks are ignored in elements
// Hint: 20-29 use commas instead to separate each line
$ vec(a, b, c) vec(a \ b \ c) $

--- math-vec-linebreaks-trailing paged html ---
// Warning: 7-10 linebreaks are ignored in elements
// Hint: 7-10 use commas instead to separate each line
$ vec(a \ ) $

--- math-vec-delim-class paged html ---
// Test that delimiters have opening and closing math class.
$ 2vec(a, delim: bar.v) 2 $
$ 2 vec(a, delim: bar.v)2 $
//@@ file tests/suite/model_asset.typ
--- asset bundle ---
#asset("data.json", "[1, 2]")

--- asset-read bundle ---
#asset("public/zoo.json", read("/assets/data/zoo.json"))

--- asset-outside-of-bundle paged ---
// Error: 2-20 assets are only supported in the bundle target
// Hint: 2-20 try enabling the bundle target
#asset("a.txt", "")

--- asset-nested bundle ---
// The error here is not ideal ...
#document("a.pdf")[
  // Error: 4-22 assets are only supported in the bundle target
  // Hint: 4-22 try enabling the bundle target
  #asset("a.txt", "")
]

--- asset-query bundle ---
#document("main.pdf")[
  = A
  = B
  = C
]
#context asset(
  "data.json",
  json.encode(query(heading).map(it => it.body.text)),
)

--- asset-log bundle ---
#let log(..s) = [#metadata(s.pos().join("", default: ""))<log>]

#let compute-and-display(x) = {
  log("Start")
  let x = 1
  for _ in range(5) {
    x += x
    log("x = ", str(x))
  }
  log("End")
  [x is #x]
}

#document("index.html", compute-and-display(5))

#context asset(
  "log.txt",
  query(<log>).map(it => it.value).join("\n", default: ""),
)
//@@ file tests/suite/model_bibliography.typ
// Test citations and bibliographies.

--- bibliography-basic paged html pdftags ---
#set pdf(standard: "ua-1")
#show: it => context { set page(width: 200pt) if target() == "paged"; it }

= Details
See also @arrgh #cite(<distress>, supplement: [p.~22]), @arrgh[p.~4], and @distress[p.~5].
#bibliography("/assets/bib/works.bib")

--- bibliography-no-title paged html ---
@distress
#bibliography("/assets/bib/works.bib", title: none)

--- bibliography-custom-title paged html ---
@distress
#bibliography("/assets/bib/works.bib", title: [My References])

--- bibliography-before-content paged ---
// Test unconventional order.
#set page(width: 200pt)
#bibliography(
  "/assets/bib/works.bib",
  title: [Works to be cited],
  style: "chicago-author-date",
)
#line(length: 100%)

As described by #cite(<netwok>, form: "prose"),
the net-work is a creature of its own.
This is close to piratery! @arrgh
And quark! @quark

--- bibliography-multiple-files paged html ---
#show: it => context { set page(width: 200pt) if target() == "paged"; it }

#set heading(numbering: "1.")
#show bibliography: set heading(numbering: "1.")

= Multiple Bibs
Now we have multiple bibliographies containing @glacier-melt @keshav2007read
#bibliography(("/assets/bib/works.bib", "/assets/bib/works_too.bib"))

--- bibliography-source-path paged empty ---
#show heading: none
#bibliography(path("/assets/bib/works_too.bib"))

--- bibliography-source-types paged empty ---
#let src = ```yaml
hi:
  type: Book
```

#show heading: none
#bibliography((
  "/assets/bib/works.bib",
  path("/assets/bib/works_too.bib"),
  bytes(src.text)
))

--- bibliography-duplicate-key eval ---
// Error: 15-65 duplicate bibliography keys: netwok, issue201, arrgh, quark, distress, glacier-melt, tolkien54, DBLP:books/lib/Knuth86a, sharing, restful, mcintosh_anxiety, psychology25
#bibliography(("/assets/bib/works.bib", "/assets/bib/works.bib"))

--- bibliography-ordering paged ---
#set page(width: 300pt)

@mcintosh_anxiety
@psychology25

#bibliography("/assets/bib/works.bib")

--- bibliography-full paged ---
#set page(paper: "a6", height: auto)
#bibliography("/assets/bib/works_too.bib", full: true)

--- bibliography-math paged ---
#set page(width: 200pt)

@Zee04
#bibliography("/assets/bib/works_too.bib", style: "mla")

--- bibliography-grid-par paged ---
// Ensure that a grid-based bibliography does not produce paragraphs.
#show par: highlight

@Zee04
@keshav2007read

#bibliography("/assets/bib/works_too.bib")

--- bibliography-indent-par paged ---
// Ensure that an indent-based bibliography does not produce paragraphs.
#show par: highlight

@Zee04
@keshav2007read

#bibliography("/assets/bib/works_too.bib", style: "mla")

--- bibliography-hanging-indent paged ---
// Test that the hanging indent of a bibliography (for styles that use one,
// like MLA) can be customized with a show-set rule for `par`, and that it
// does not leak into paragraphs outside of the bibliography.
#show bibliography: set par(hanging-indent: 3em)

Hanging indent should not affect this paragraph, which is long enough to wrap
onto a second line.

@Zee04
@keshav2007read

#bibliography("/assets/bib/works_too.bib", style: "mla")

--- bibliography-style-not-suitable paged ---
// Error: 2-62 CSL style "Alphanumeric" is not suitable for bibliographies
#bibliography("/assets/bib/works.bib", style: "alphanumeric")

--- bibliography-empty-key eval ---
#let src = ```yaml
"":
  type: Book
```
// Error: 15-30 bibliography contains entry with empty key
#bibliography(bytes(src.text))

--- issue-4618-bibliography-set-heading-level paged ---
// Test that the bibliography block's heading is set to 2 by the show rule,
// and therefore should be rendered like a level-2 heading. Notably, this
// bibliography heading should not be underlined.
#show heading.where(level: 1): it => [ #underline(it.body) ]
#show bibliography: set heading(level: 2)

= Level 1
== Level 2
@Zee04

#bibliography("/assets/bib/works_too.bib")

--- bibliography-chicago-fullnotes-warning paged empty ---
// Test warning for deprecated alias.
// Warning: 47-66 style `"chicago-fullnotes"` has been deprecated in favor of `"chicago-notes"`
#bibliography("/assets/bib/works.bib", style: "chicago-fullnotes", title: none)

--- bibliography-modern-humanities-research-association-warning paged empty ---
// Test warning for deprecated alias.
// Warning: 47-87 style `"modern-humanities-research-association"` has been deprecated in favor of `"modern-humanities-research-association-notes"`
#bibliography("/assets/bib/works.bib", style: "modern-humanities-research-association", title: none)

--- bibliography-csl-display paged html ---
// Test a combination of CSL `display` attributes. Most of the display
// attributes are barely used by any styles, so we have a custom style here.

#let style = ```csl
  <?xml version="1.0" encoding="utf-8"?>
  <style xmlns="http://purl.org/net/xbiblio/csl" class="in-text" version="1.0">
    <info>
      <title>Test</title>
      <id>test</id>
    </info>
    <citation collapse="citation-number">
      <layout>
        <text variable="citation-number"/>
      </layout>
    </citation>
    <bibliography>
      <layout>
        <text variable="title" font-style="italic" />
        <text variable="citation-number" display="left-margin" prefix="|" suffix="|" />
        <group display="indent">
          <text term="by" suffix=" " />
          <!-- This left-margin attribute is ignored because it is in a container. -->
          <names variable="author" display="left-margin" />
        </group>
        <group display="block" prefix="(" suffix=")">
          <text term="edition" suffix=" " text-case="capitalize-first" />
          <date variable="issued"><date-part name="year"/></date>
        </group>
      </layout>
    </bibliography>
  </style>
```

#let bib = ```bib
  @article{entry1,
    title={Title 1},
    author={Author 1},
    year={2021},
  }
```

#bibliography(
  bytes(bib.text),
  style: bytes(style.text),
  title: none,
  full: true,
)

--- bibliography-group-none html ---
#set bibliography(group: none)

@keshav2007read @netwok @arrgh
#bibliography("/assets/bib/works.bib")
#bibliography("/assets/bib/works_too.bib", style: "nlm-citation-sequence")

--- bibliography-group-auto html ---
@netwok @arrgh @keshav2007read
#bibliography("/assets/bib/works.bib")
#bibliography("/assets/bib/works_too.bib")

--- bibliography-group-str html ---
@netwok @arrgh @keshav2007read
#bibliography("/assets/bib/works.bib", group: "a")
#bibliography("/assets/bib/works_too.bib", group: "b", style: "nlm-citation-sequence")

@quark @distress @keshav2007read
#bibliography("/assets/bib/works.bib", group: "a")
#bibliography("/assets/bib/works_too.bib", group: "b", style: "nlm-citation-sequence")

--- bibliography-group-full html ---
// Test that a numbered bibliography that is printed in full without any
// citations still advances the shared numbering.
#bibliography("/assets/bib/works_too.bib", full: true)
#bibliography("/assets/bib/works.bib", full: true)

--- bibliography-group-mixed-full html ---
// Test that the shared numbering continues from a bibliography with
// `full: true` into one that only contains cited entries, and that the
// latter advances the numbering by its rendered items rather than by the
// entries in its source file.
#bibliography("/assets/bib/works_too.bib", full: true)

@netwok
#bibliography("/assets/bib/works.bib")

#bibliography("/assets/bib/works_too.bib", full: true)

--- bibliography-group-mixed-styles html ---
// Test that a bibliography with a non-numeric style does not consume
// numbers from the shared numbering.
@Zee04
#bibliography("/assets/bib/works_too.bib", style: "apa")

@netwok
#bibliography("/assets/bib/works.bib")

--- bibliography-group-citation-numbers-only html ---
// Test that a bibliography whose style displays citation numbers in
// citations, but not in the bibliography itself, still advances the
// shared numbering.
#let style = ```csl
  <?xml version="1.0" encoding="utf-8"?>
  <style xmlns="http://purl.org/net/xbiblio/csl" class="in-text" version="1.0">
    <info>
      <title>Test</title>
      <id>test</id>
    </info>
    <citation>
      <layout prefix="[" suffix="]" delimiter=", ">
        <text variable="citation-number"/>
      </layout>
    </citation>
    <bibliography>
      <layout>
        <text variable="title"/>
      </layout>
    </bibliography>
  </style>
```
#set bibliography(style: bytes(style.text))

@Zee04
#bibliography("/assets/bib/works_too.bib")

@netwok
#bibliography("/assets/bib/works.bib")

--- bibliography-group-form-year html ---
// Test that a bibliography whose citations all use a form that displays no
// citation number still advances the shared numbering.
#cite(<Zee04>, form: "year")
#bibliography("/assets/bib/works_too.bib")

@netwok
#bibliography("/assets/bib/works.bib")

--- bibliography-group-full-custom-style html ---
// Test that the shared numbering also advances for a numbered custom style
// that does not declare its citation format.
#let style = ```csl
  <?xml version="1.0" encoding="utf-8"?>
  <style xmlns="http://purl.org/net/xbiblio/csl" class="in-text" version="1.0">
    <info>
      <title>Test</title>
      <id>test</id>
    </info>
    <citation>
      <layout prefix="[" suffix="]" delimiter=", ">
        <text variable="citation-number"/>
      </layout>
    </citation>
    <bibliography second-field-align="flush">
      <layout>
        <text variable="citation-number" prefix="[" suffix="]"/>
        <text variable="title"/>
      </layout>
    </bibliography>
  </style>
```

#set bibliography(style: bytes(style.text), full: true)
#bibliography("/assets/bib/works_too.bib")
#bibliography("/assets/bib/works.bib")

--- bibliography-group-out-of-order html ---
// The numbers are ordered in the order of the bibliographies, so in the prose
// they can be out of order if using shared numbering.
@keshav2007read @netwok @arrgh
#bibliography("/assets/bib/works.bib")
#bibliography("/assets/bib/works_too.bib")

--- bibliography-group-sorted-style html ---
// Test that the shared numbering also advances for a style that uses sorting.
#set bibliography(style: "association-for-computing-machinery")
@netwok @keshav2007read
#bibliography("/assets/bib/works.bib")
#bibliography("/assets/bib/works_too.bib")
//@@ file tests/suite/model_cite.typ
--- cite-footnote paged ---
Hello @netwok
And again: @netwok

#pagebreak()
#bibliography("/assets/bib/works.bib", style: "chicago-shortened-notes")

--- cite-form paged html ---
#show: it => context { set page(width: 200pt) if target() == "paged"; it }

Nothing: #cite(<arrgh>, form: none)

#cite(<netwok>, form: "prose") say stuff.

#bibliography("/assets/bib/works.bib", style: "apa")

--- cite-group paged html ---
A#[@netwok@arrgh]B \
A@netwok@arrgh B \
A@netwok @arrgh B \
A@netwok @arrgh. B \

A @netwok#[@arrgh]B \
A @netwok@arrgh, B \
A @netwok @arrgh, B \
A @netwok @arrgh. B \

A#[@netwok @arrgh @quark]B. \
A @netwok @arrgh @quark B. \
A @netwok @arrgh @quark, B.

#show bibliography: it => if target() == "html" { it }
#bibliography("/assets/bib/works.bib", style: "american-physics-society")

--- cite-grouping-and-ordering paged ---
@mcintosh_anxiety
@psychology25
@netwok
@issue201
@arrgh
@quark
@distress,
@glacier-melt
@issue201
@tolkien54
@sharing
@restful

#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "american-physics-society")

--- issue-785-cite-locate paged ---
// Test citation in other introspection.
#set page(width: 180pt)
#set heading(numbering: "1.")

#outline(
  title: [Figures],
  target: figure.where(kind: image),
)

#pagebreak()

= Introduction <intro>
#figure(
  rect(height: 10pt),
  caption: [A pirate @arrgh in @intro],
)

#context [Citation @distress on page #here().page()]

#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "chicago-shortened-notes")

--- issue-1597-cite-footnote paged ---
// Tests that when a citation footnote is pushed to next page, things still
// work as expected.
#set page(height: 60pt)
A

#footnote[@netwok]
#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- issue-2531-cite-show-set paged ---
// Test show set rules on citations.
#show cite: set text(red)
A @netwok @arrgh.
B #cite(<netwok>) #cite(<arrgh>).

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- issue-3481-cite-location paged ---
// The locator was cloned in the wrong location, leading to inconsistent
// citation group locations in the second footnote attempt.
#set page(height: 60pt)

// First page shouldn't be empty because otherwise we won't skip the first
// region which causes the bug in the first place.
#v(10pt)

// Everything moves to the second page because we want to keep the line and
// its footnotes together.
#footnote[@netwok \ A]

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- issue-3699-cite-twice-et-al paged ---
// Citing a second time showed all authors instead of "et al".
@mcintosh_anxiety \
@mcintosh_anxiety
#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "chicago-author-date")

--- issue-5503-cite-in-align paged ---
// The two aligned elements should be displayed in separate lines.
#align(right)[@netwok]
#align(right)[b]

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- issue-5503-cite-group-interrupted-by-par-align paged ---
// `par` and `align` are block-level and should interrupt a cite group
@netwok
@arrgh
#par(leading: 5em)[@netwok]
#par[@arrgh]
@netwok
@arrgh
#align(right)[@netwok]
@arrgh

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- cite-type-error-hint eval ---
// Test hint for cast error from str to label
// Error: 7-15 expected label, found string
// Hint: 7-15 use `<netwok>` or `label("netwok")` to create a label
#cite("netwok")

--- cite-type-error-hint-invalid-literal eval ---
// Test hint for cast error from str to label
// Error: 7-17 expected label, found string
// Hint: 7-17 use `label("%@&#*!\\")` to create a label
#cite("%@&#*!\\")

--- issue-5775-cite-order-rtl paged ---
// Test citation order in RTL text.
#set page(width: 300pt)
#set text(font: ("Libertinus Serif", "Noto Sans Arabic"))
@netwok
aaa
این است
@tolkien54
و این یکی هست
@arrgh

#bibliography("/assets/bib/works.bib")

--- cite-chicago-fullnotes-warning paged ---
// Test warning for deprecated alias.
// Warning: 24-43 style `"chicago-fullnotes"` has been deprecated in favor of `"chicago-notes"`
#cite(<netwok>, style: "chicago-fullnotes")
#bibliography("/assets/bib/works.bib")

--- cite-chicago-fullnotes-set-rule-warning paged empty ---
// Test warning for deprecated alias.
// Warning: 18-37 style `"chicago-fullnotes"` has been deprecated in favor of `"chicago-notes"`
#set cite(style: "chicago-fullnotes")

--- cite-supplements-and-ibid paged ---
#set page(width: 300pt)

Par 1 @arrgh

Par 2 @arrgh[p. 5-8]

Par 3 @arrgh[p. 5-8]

Par 4 @arrgh[p. 9-10]

Par 5 @arrgh[*p. 9-10*]

Par 6 @arrgh[*p. 9-10*]

#let style = bytes(
  ```xml
  <?xml version="1.0" encoding="utf-8"?>
  <style xmlns="http://purl.org/net/xbiblio/csl" version="1.0" class="note" default-locale="pl-PL">
    <info>
      <title>Example citation style</title>
      <id>http://www.example.com/</id>
    </info>
    <macro name="locator">
      <group delimiter=" ">
        <label variable="locator" form="short"/>
        <text variable="locator"/>
      </group>
    </macro>

    <citation>
      <sort>
        <key variable="title"/>
      </sort>
      <layout>
        <choose>
          <if position="first">
            <group delimiter=", ">
              <text variable="title"/>
              <text macro="locator"/>
            </group>
          </if>
          <else-if position="ibid-with-locator">
            <group delimiter=", ">
              <text term="ibid"/>
              <text macro="locator"/>
            </group>
          </else-if>
          <else-if position="ibid">
            <text term="ibid"/>
          </else-if>
          <else-if position="subsequent">
            <group delimiter=", ">
              <text variable="title"/>
              <text macro="locator"/>
            </group>
          </else-if>
        </choose>
      </layout>
    </citation>

    <bibliography>
      <sort>
        <key variable="title"/>
      </sort>
      <layout>
        <text variable="title"/>
      </layout>
    </bibliography>
  </style>
  ```.text
)

#bibliography("/assets/bib/works.bib", style: style)

--- cite-no-bib paged ---
// Error: 2-19 the document does not contain a bibliography
#cite(<something>)

--- cite-unknown paged ---
// Error: 2-19 citation key `something` is not present in the bibliography
#cite(<something>)
#bibliography("/assets/bib/works.bib")

--- cite-unknown-multi-bib paged ---
// Error: 2-19 citation key `something` is not present in any bibliography
#cite(<something>)
#bibliography("/assets/bib/works.bib")
#bibliography("/assets/bib/works_too.bib")

--- cite-uncovered paged ---
// Error: 1-8 citation is not covered by any bibliography
@netwok
// Hint: 1:10-4:2 a bibliography containing the key `netwok` exists, but its `target` excludes this citation
#context bibliography(
  "/assets/bib/works.bib",
  target: selector(cite).after(here()),
)

--- cite-nonexistent paged ---
// Error: 2-14 key `baum` does not exist in the bibliography
#cite(<baum>)
// Hint: 2-72 the citation was assigned to this bibliography
#bibliography("/assets/bib/works.bib", target: cite.where(key: <baum>))

--- cite-assignment-by-chapter html ---
#show bibliography: set heading(offset: 1)

= Pirates
@netwok @arrgh
#bibliography("/assets/bib/works.bib")

= Glaciers
@glacier-melt
#bibliography("/assets/bib/works.bib")

--- cite-assignment-auto html ---
#show bibliography: set heading(numbering: "(A)")
@netwok // bib (C)
#bibliography("/assets/bib/works_too.bib")
#bibliography("/assets/bib/works.bib", target: cite.where(key: <never>))
#bibliography("/assets/bib/works.bib")
@keshav2007read // bib (A)
@arrgh // bib (D)
#bibliography("/assets/bib/works.bib")

--- cite-assignment-target html ---
#bibliography(
  "/assets/bib/works.bib",
  title: [The middle],
  target: selector(cite).after(<b>).before(<c>),
  style: "mla",
)

= A <a>
@netwok @arrgh

= B <b>
@netwok
@arrgh

= C <c>
@distress

#bibliography(
  "/assets/bib/works.bib",
  title: [The rest],
)

--- cite-assignment-mixed-group html ---
// There's a space between [2] and (Keshav), but none between (Keshav) and [3].
// Should be the same in the output.
See @netwok @arrgh @keshav2007read#cite(<quark>) @distress @tolkien54 @sharing

#show bibliography: set heading(offset: 1)
= Bibliographies
#bibliography(
  "/assets/bib/works.bib",
  title: [IEEE],
  style: "ieee",
)
#bibliography(
  "/assets/bib/works_too.bib",
  title: [MLA],
  style: "mla",
)
#bibliography(
  "/assets/bib/works.bib",
  title: [Chicago],
  style: "chicago-author-date",
  target: selector.or(cite.where(key: <tolkien54>), cite.where(key: <sharing>)),
)
//@@ file tests/suite/model_divider.typ
// Test divider element.

--- divider-basic paged ---
// Basic divider.
#set page(width: 200pt)
Before
#divider()
After

--- divider-multiple paged ---
// Test multiple dividers.
#set page(width: 200pt)
Section 1
#divider()
Section 2
#divider()
Section 3

--- divider-in-container paged ---
// Test divider in a container.
#set page(width: 200pt)
#box(width: 150pt, stroke: 1pt, inset: 10pt)[
  Content before
  #divider()
  Content after
]

--- divider-show-set-line paged ---
// Test customizing line via set rule.
#set page(width: 200pt)
#show divider: set line(stroke: 2pt + red)
Before
#divider()
After

--- divider-show-centered paged ---
// Test centered, shorter divider.
#set page(width: 200pt)
#show divider: block(
  width: 100%,
  spacing: 1em,
  align(center, line(length: 50%)),
)
Before
#divider()
After

--- divider-show-decorative paged ---
// Test replacing with custom content (asterism).
#set page(width: 200pt)
#show divider: set align(center)
#show divider: block[∗ ∗ ∗]
Chapter 1
#divider()
Chapter 2

--- divider-html html ---
// Test HTML output (should map to <hr>).
Introduction
#divider()
Body text
//@@ file tests/suite/model_document.typ
// Test document and page-level styles.

--- document-set-title paged ---
#set document(title: [Hello])
What's up?

--- document-set-author-date paged empty ---
#set document(author: ("A", "B"), date: datetime.today())

--- document-date-bad eval ---
// Error: 21-28 expected datetime, none, or auto, found string
#set document(date: "today")

--- document-author-bad eval ---
// Error: 23-29 expected string, found integer
#set document(author: (123,))
What's up?

--- document-set-after-content paged ---
// Document set rules can appear anywhere in top-level realization, also after
// content.
Hello
#set document(title: [Hello])

--- document-constructor bundle ---
#document("a.pdf")[This is a PDF document]

--- document-constructor-incomplete eval ---
// Error: 2-12 missing argument: path
#document()

--- document-path-is-root-1 eval ---
// Error: 11-14 path must have at least one component
#document("/", format: "pdf")[Root?]

--- document-path-is-root-2 eval ---
// Error: 11-15 path must have at least one component
#document("/.", format: "pdf")[Root?]

--- document-path-escapes eval ---
// Error: 11-15 path `".."` would escape the bundle root
#document("..", format: "pdf")[Root?]

--- document-multiple-authors-html html ---
// Should emit multiple `<meta>` tags.
#set document(author: ("John Doe", "Jane Doe"))

--- document-multiple-keywords-html html ---
// Should emit a single `<meta>` tag.
#set document(keywords: ("foo", "bar"))

--- document-weird-extension bundle ---
// The hashes of both files should match.
#document("normal.pdf")[A PDF]
#document("weird.oops", format: "pdf")[A PDF]

--- document-unknown-format-inferred bundle ---
// Error: 2-21 unknown document format
// Hint: 2-21 try specifying the `format` explicitly
#document("a.txt")[]

--- document-unknown-format-specified eval ---
// Error: 28-33 expected "pdf", "png", "svg", "html", or auto
#document("a.pdf", format: "txt")[]

--- document-path-collision bundle ---
// Hint: 2-26 path is already in use here
#document("a.pdf")[Doc A]
// Hint: 2-26 path is already in use here
// Hint: 2-26 path is already in use here
#document("b.pdf")[Doc A]
// Error: 2-27 path `a.pdf` occurs multiple times in the bundle
// Hint: 2-27 document paths must be unique in the bundle
#document("a.pdf")[Doc A']
// Error: 2-27 path `b.pdf` occurs multiple times in the bundle
// Hint: 2-27 document paths must be unique in the bundle
#document("b.pdf")[Doc B']
// Error: 2-28 path `b.pdf` occurs multiple times in the bundle
// Hint: 2-28 asset paths must be unique in the bundle
#asset("b.pdf", "Fake PDF")
#document("c.pdf")[Doc C]

--- document-outside-of-bundle paged ---
// Error: 2-21 constructing a document is only supported in the bundle target
// Hint: 2-21 try enabling the bundle target
// Hint: 2-21 or use a `set document(..)` rule to configure metadata
#document("a.pdf")[]

--- document-format-outside-of-bundle paged html ---
// Error: 2-30 setting the document format is only supported in the bundle target
#set document(format: "html")

--- document-format-in-container paged html ---
#block[
  // Error: 4-32 document set rules are not allowed inside of containers
  #set document(format: "html")
]

--- document-nested bundle ---
// The error here is not ideal ...
#document("a.pdf")[
  // Error: 4-23 constructing a document is only supported in the bundle target
  // Hint: 4-23 try enabling the bundle target
  // Hint: 4-23 or use a `set document(..)` rule to configure metadata
  #document("c.pdf")[]
]

--- document-image-multi-page bundle ---
#let multi = [
  #set page(height: 100pt)
  #rect(height: 100pt)
  #rect(height: 100pt)
]

// Error: 2-30 expected document to have a single page
// Hint: 2-30 the document resulted in 2 pages
// Hint: 2-30 documents exported to an image format only support a single page
#document("image.svg", multi)

// Error: 2-30 expected document to have a single page
// Hint: 2-30 the document resulted in 2 pages
// Hint: 2-30 documents exported to an image format only support a single page
#document("image.png", multi)

--- document-realization-errors bundle ---
// This test ensures that we show errors from all document realizations at once.

// Error: 30-37 cannot add integer and string
#document("bar.pdf", context 1 + "2")
// Error: 23-27 label `<bar>` does not exist in the document
#document("foo.pdf", [@bar])

--- document-export-errors bundle ---
// This test ensures that we show errors from all exports at once when trying to
// create a bundle.

// This document is fine.
#document(
  "foo.html",
  [Hello],
)

// This one errors.
#document(
  "bar.html",
  // Error: 3-27 HTML raw text element cannot contain its own closing tag
  // Hint: 3-27 the sequence `</script` appears in the raw text
  html.script("</script>"),
)

// This one errors, too.
#document(
  "baz.html",
  // Error: 3-25 HTML raw text element cannot contain its own closing tag
  // Hint: 3-25 the sequence `</style` appears in the raw text
  html.style("</style>"),
)

--- document-content-without-document bundle ---
// Error: 1-8 heading is not allowed at the top-level in bundle export
// Hint: 1-8 try wrapping the content in a `document` instead
= Hello
// Error: 1-6 text is not allowed at the top-level in bundle export
// Hint: 1-6 try wrapping the content in a `document` instead
world

--- document-properties-in-bundle bundle ---
// Ensures that the title element works correctly in bundle export.
// See the comment in `impl ShowSet for Packed<DocumentElem>` for more details.
#document(
  "hi.html",
  title: [Test title],
  author: "Test Author",
  description: [Test description],
  keywords: ("a", "b"),
)[
  #title()
  // Testing all properties separately because, in the current implementation,
  // they are manually "forwarded".
  #context {
    test(document.format, auto)
    test(document.title, [Test title])
    test(document.author, ("Test Author",))
    test(document.description, [Test description])
    test(document.keywords, ("a", "b"))
    test(document.date, auto)
  }
]

--- document-properties-precedence bundle ---
// Tests the precedence of different ways to specify document metadata. Each
// subcase's filename specifies the expected metadata output (for HTML, in the
// tab bar) and printed output (what `#title()` displays), in that order.

// Just an arg of course works.
#document("1-arg-arg.html", title: [Arg], title())

// Just a set rule, too.
#for ext in ("html", "pdf") {
  set document(title: [Outer])
  document("2-outer-outer." + ext, title())
}

// An explicit arg wins against a set rule, as usual.
#{
  set document(title: [Outer])
  document("3-arg-arg.html", title: [Arg], title())
}

// An interior set rule is also supported. While that's not the usual behavior
// of Typst, this is how document metadata is configured in single-document
// export. Supporting it in bundles, too, makes it easier to compose bundle
// export with documents that also work standalone, e.g. something like
// `document("paper.pdf", include "paper.typ")`.
#document("4-inner-inner.html", {
  set document(title: [Inner])
  title()
})

// An interior set rule wins against an exterior one. The outer set rule can be
// viewed similarly to the global default in its effect, so given that we want
// to support interior `set document` at all, the inner one should win here.
// Otherwise, the inner one wouldn't win against the global default either,
// which makes no sense.
#{
  set document(title: [Outer])
  document("5-inner-inner.html", {
    set document(title: [Inner])
    title()
  })
}

// An interior set rule wins even against an explicit argument. This is a
// little odd, but letting `document(title: ..)` have precedence is also
// problematic, for two reasons:
//
// - By the time we're compiling the document, we can't distinguish whether
//   `[Arg]` is an inherent property, came from a set rule, or is just the
//   global default because the document element is already materialized. So if
//   we always used that materialized property, an inner `set document` wouldn't
//   ever have a chance to work. We could accept that, but having the title work
//   in something like `document("paper.pdf", include "paper.typ")` where the
//   paper already has a document set rule is too useful to ignore.
//
// - Even if we could make `[Arg]` win here, it would in turn be somewhat odd if
//   moving bundle-level explicit arguments to a bundle-level set rule would not
//   work (because `[Inner]` would definitely win against `[Outer]` if we want
//   to support inner set document rules at all; see the `5-inner-inner.html`
//   case).
//
// With the chosen implementation, all of bundle-level set rules, explicit
// document constructor arguments, and set document rules within the document
// work cleanly if used in isolation. The only slighty quirky behavior is that
// an inner `set document` wins against an explicit argument.
#for ext in ("html", "pdf") {
  set document(title: [Outer])
  document("6-inner-inner." + ext, title: [Arg])[
    #set document(title: [Inner])
    #title()
  ]
}

--- document-property-bad-in-bundle bundle ---
// This does not work because it's a required argument and not a settable one.
// Error: 39-43 function `document` does not contain field `path`
#document("hi.html", context document.path)

--- document-set-in-container paged ---
#box[
  // Error: 4-32 document set rules are not allowed inside of containers
  #set document(title: [Hello])
]

--- issue-4065-document-context paged empty ---
// Test that we can set document properties based on context.
#show: body => context {
  let all = query(heading)
  let title = if all.len() > 0 { all.first().body }
  set document(title: title)
  body
}

#show heading: none
= Top level

--- issue-4769-document-context-conditional paged empty ---
// Test that document set rule can be conditional on document information
// itself.
#set document(author: "Normal", title: [Alternative])
#context {
  set document(author: "Changed") if "Normal" in document.author
}

--- issue-8163-document-path-state bundle ---
// Test that it's possible to use state/context for determining a document's
// file path.
#let c = counter("doc-name")
#let counted-document(body) = {
  context {
    let i = c.get().first() + 1
    document("doc-"  + str(i) + ".html")[
      This is document #i with content #body
    ]
  }
  c.step()
}

#counted-document[A]
#counted-document[B]

--- issue-8616-document-delayed-pages bundle ---
// Test that bundle documents which are expected to have exactly one page
// perform this assertion only on the final iteration.
#import "../introspection/switch.typ": switch
#for ext in ("png", "svg") {
  document("doc." + ext, switch(n => if n == 1 { pagebreak() }))
}
//@@ file tests/suite/model_emph-strong.typ
// Test emph and strong.

--- emph-syntax paged ---
// Basic.
_Emphasized and *strong* words!_

// Inside of a word it's a normal underscore or star.
hello_world Nutzer*innen

// CJK characters will not need spaces.
中文一般使用*粗体*或者_楷体_来表示强调。

日本語では、*太字*や_斜体_を使って強調します。

中文中混有*Strong*和_Emphasis_。

// Can contain paragraph in nested content block.
_Still #[

] emphasized._

--- emph-and-strong-call-in-word paged ---
// Inside of words can still use the functions.
P#strong[art]ly em#emph[phas]ized.

--- emph-empty-hint paged empty ---
// Warning: 1-3 no text within underscores
// Hint: 1-3 using multiple consecutive underscores (e.g. __) has no additional effect
__

--- emph-double-underscore-empty-hint paged ---
// Warning: 1-3 no text within underscores
// Hint: 1-3 using multiple consecutive underscores (e.g. __) has no additional effect
// Warning: 13-15 no text within underscores
// Hint: 13-15 using multiple consecutive underscores (e.g. __) has no additional effect
__not italic__

--- emph-unclosed eval ---
// Error: 6-7 unclosed delimiter
#box[_Scoped] to body.

--- emph-ends-at-parbreak eval ---
// Ends at paragraph break.
// Error: 1-2 unclosed delimiter
_Hello

World

--- emph-strong-unclosed-nested eval ---
// Error: 11-12 unclosed delimiter
// Error: 3-4 unclosed delimiter
#[_Cannot *be interleaved]

--- strong-delta paged ---
// Adjusting the delta that strong applies on the weight.
Normal

#set strong(delta: 300)
*Bold*

#set strong(delta: 150)
*Medium* and *#[*Bold*]*

--- strong-empty-hint paged empty ---
// Warning: 1-3 no text within stars
// Hint: 1-3 using multiple consecutive stars (e.g. **) has no additional effect
**

--- strong-double-star-empty-hint paged ---
// Warning: 1-3 no text within stars
// Hint: 1-3 using multiple consecutive stars (e.g. **) has no additional effect
// Warning: 11-13 no text within stars
// Hint: 11-13 using multiple consecutive stars (e.g. **) has no additional effect
**not bold**
//@@ file tests/suite/model_enum.typ
// Test enumerations.

--- enum-function-call paged ---
#enum[Embrace][Extend][Extinguish]

--- enum-number-override-nested paged pdftags ---
#set pdf(standard: "ua-1")
0. Before first!
1. First.
   2. Indented

+ Second

--- enum-built-in-loop paged ---
// Test automatic numbering in summed content.
#for i in range(5) {
   [+ #numbering("I", 1 + i)]
}

--- list-mix paged ---
// Mix of different lists
- Bullet List
+ Numbered List
/ Term: List

--- enum-syntax-at-start paged ---
// In the line.
1.2 \
This is 0. \
See 0.3. \

--- enum-syntax-edge-cases paged ---
// Edge cases.
+
Empty \
+Nope \
a + 0.

--- enum-syntax-number-length paged ---
// Ensure that indentation works from the beginning of a number, not the end.

10. a
   11. b
 12. c // same level as b
  13. d // indented past c
14. e

--- enum-in-align-in-block paged ---
// The marker doesn't move as the list body expands and aligns itself.
// However, with `block`, the list body does not expand, so the marker is also
// aligned.
+ a
+ b
#align(right)[+ c]
#align(right, block[+ d])

--- enum-in-align-centering paged ---
+ $ x $
#align(right)[+ $ y $]
#align(right, block[+ $ z $])

--- enum-number-override paged ---
// Test item number overriding.
1. first
+ second
5. fifth

#enum(
   enum.item(1)[First],
   [Second],
   enum.item(5)[Fifth]
)

--- enum-item-number-optional paged ---
#enum.item[First]
#enum.item[Second]

--- enum-numbering-pattern paged ---
// Test numbering pattern.
#set enum(numbering: "(1.a.*)")
+ First
+ Second
  2. Nested
     + Deep
+ Normal

--- enum-numbering-full paged ---
// Test full numbering.
#set enum(numbering: "1.a.", full: true)
+ First
  + Nested

--- enum-numbering-reversed paged ---
// Test reverse numbering.
#set enum(reversed: true)
+ Coffee
+ Tea
+ Milk

--- enum-numbering-reversed-overridden paged ---
// Test reverse numbering with overridden numbers.
#set enum(reversed: true)
+ A
+ B
+ C
9. D
+ E
+ F

--- enum-numbering-closure paged ---
// Test numbering with closure.
#enum(
  start: 3,
  spacing: 0.65em - 3pt,
  tight: false,
  numbering: n => text(
    fill: (red, green, blue).at(calc.rem(n, 3)),
    numbering("A", n),
  ),
  [Red], [Green], [Blue], [Red],
)

--- enum-start html ---
#enum(
  start: 3,
  [Skipping],
  [Ahead],
)

--- enum-numbering-closure-nested paged ---
// Test numbering with closure and nested lists.
#set enum(numbering: n => super[#n])
+ A
  + B
+ C

--- enum-numbering-closure-nested-complex paged ---
// Test numbering with closure and nested lists.
#set text(font: "New Computer Modern")
#set enum(numbering: (..args) => math.mat(args.pos()), full: true)
+ A
  + B
  + C
    + D
+ E
+ F

--- enum-numbering-pattern-empty eval ---
// Error: 22-24 invalid numbering pattern
#set enum(numbering: "")

--- enum-numbering-pattern-invalid eval ---
// Error: 22-28 invalid numbering pattern
#set enum(numbering: "(())")

--- enum-numbering-huge paged ---
// Test values greater than 32-bits
100000000001. A
+             B

--- enum-number-align-unaffected paged ---
// Alignment shouldn't affect number
#set align(horizon)

+ ABCDEF\ GHIJKL\ MNOPQR
   + INNER\ INNER\ INNER
+ BACK\ HERE

--- enum-number-align-default paged ---
// Enum number alignment should be 'end' by default
1. a
10. b
100. c

--- enum-number-align-specified paged ---
#set enum(number-align: start)
1.  a
8.  b
16. c

--- enum-number-align-2d paged ---
#set enum(number-align: center + horizon)
1.  #box(fill: teal, inset: 10pt )[a]
8.  #box(fill: teal, inset: 10pt )[b]
16. #box(fill: teal,inset: 10pt )[c]

--- enum-number-align-unfolded paged ---
// Number align option should not be affected by the context.
#set align(center)
#set enum(number-align: start)

4.  c
8.  d
16. e\ f
   2.  f\ g
   32. g
   64. h

--- enum-number-align-unfolded-mixed paged ---
// Verify whether overriding vertical alignment causes horizontal alignment to
// be inherited from the context.
#set align(center)
#set enum(
  number-align: top,
  numbering: n => "1" * n,
)

+ abc
+ abc
+ abc

--- enum-number-align-values paged empty ---
// Test valid number align values (horizontal and vertical)
#set enum(number-align: start)
#set enum(number-align: end)
#set enum(number-align: left)
#set enum(number-align: center)
#set enum(number-align: right)
#set enum(number-align: top)
#set enum(number-align: horizon)
#set enum(number-align: bottom)

--- enum-par paged html ---
// Check whether the contents of enum items become paragraphs.
#show par: it => if target() != "html" { highlight(it) } else { it }

// No paragraphs.
#block[
  + Hello
  + World
]

#block[
  + Hello // Paragraphs

    From
  + World // No paragraph because it's a tight enum
]

#block[
  + Hello // Paragraphs

    From

    The

  + World // Paragraph because it's a wide enum
]

--- issue-2530-enum-item-panic paged ---
// Enum item (pre-emptive)
#enum.item(auto)[Hello]
#enum.item(17)[Hello]

--- issue-5503-enum-in-align paged ---
// `align` is block-level and should interrupt an enum.
+ a
+ b
#align(right)[+ c]
+ d

--- issue-5719-enum-nested paged ---
// Enums can be immediately nested.
1. A
2. 1. B
   2. C
+ + D
  + E
+ = F
  G

--- issue-529-enum-center-alignment paged ---
#set page(width: 2cm)
+ A
+ #align(center)[B]
+ $ C $

--- issue-1204-enum-baseline-alignment paged ---
+ A
+ $ sum_(i = 1)^n overbrace(x^6, y) $
+ #box(baseline: 1cm)[C]
+ #v(1cm) D
+ #text(48pt)[E]
+ #block(inset: 10pt, stroke: red)[Hello world!]
+ #rect[Hello world!]
//@@ file tests/suite/model_figure.typ
// Test figures.

--- figure-basic paged pdftags ---
#set pdf(standard: "ua-1")
#set page(width: 150pt)
#set figure(numbering: "I")

We can clearly see that @fig-cylinder and
@tab-complex are relevant in this context.

#figure(
  table(columns: 2)[a][b],
  caption: [The basic table.],
) <tab-basic>

#figure(
  pad(y: -6pt, image(alt: "Sketch of white standing cylinder", "/assets/images/cylinder.svg", height: 2cm)),
  caption: [The basic shapes.],
  numbering: "I",
) <fig-cylinder>

#figure(
  table(columns: 3)[a][b][c][d][e][f],
  caption: [The complex table.],
) <tab-complex>

--- figure-align paged ---
#show figure: set align(start)
#figure(
  rect[This is \ left],
  caption: [Start-aligned]
)

--- figure-table paged ---
// Testing figures with tables.
#figure(
  table(
    columns: 2,
    [Second cylinder],
    image("/assets/images/cylinder.svg"),
  ),
  caption: "A table containing images."
) <fig-image-in-table>

--- figure-placement paged ---
#set page(height: 160pt, columns: 2)
#set place(clearance: 10pt)

#lines(4)

#figure(
  placement: auto,
  scope: "parent",
  caption: [I],
  rect(height: 15pt, width: 80%),
)

#figure(
  placement: bottom,
  caption: [II],
  rect(height: 15pt, width: 80%),
)

#lines(2)

#figure(
  placement: bottom,
  caption: [III],
  rect(height: 25pt, width: 80%),
)

#figure(
  placement: auto,
  scope: "parent",
  caption: [IV],
  rect(width: 80%),
)

#lines(15)

--- figure-scope-without-placement paged ---
// Error: 2-27 parent-scoped placement is only available for floating figures
// Hint: 2-27 you can enable floating placement with `figure(placement: auto, ..)`
#figure(scope: "parent")[]

--- figure-theorem paged ---
// Testing show rules with figures with a simple theorem display
#show figure.where(kind: "theorem"): it => {
  set align(start)
  let name = none
  if not it.caption == none {
    name = [ #emph(it.caption.body)]
  } else {
    name = []
  }

  let title = none
  if not it.numbering == none {
    title = it.supplement
    if not it.numbering == none {
      title += " " +  it.counter.display(it.numbering)
    }
  }
  title = strong(title)
  pad(
    top: 0em, bottom: 0em,
    block(
      fill: green.lighten(90%),
      stroke: 1pt + green,
      inset: 10pt,
      width: 100%,
      radius: 5pt,
      breakable: false,
      [#title#name#h(0.1em):#h(0.2em)#it.body#v(0.5em)]
    )
  )
}

#set page(width: 150pt)
#figure(
  $a^2 + b^2 = c^2$,
  supplement: "Theorem",
  kind: "theorem",
  caption: "Pythagoras' theorem.",
  numbering: "1",
) <fig-formula>

#figure(
  $a^2 + b^2 = c^2$,
  supplement: "Theorem",
  kind: "theorem",
  caption: "Another Pythagoras' theorem.",
  numbering: none,
) <fig-formula>

#figure(
  ```rust
  fn main() {
    println!("Hello!");
  }
  ```,
  caption: [Hello world in _rust_],
)

--- figure-breakable paged ---
// Test breakable figures
#set page(height: 6em)
#show figure: set block(breakable: true)

#figure(table[a][b][c][d][e], caption: [A table])

--- figure-caption-separator paged ---
// Test custom separator for figure caption
#set figure.caption(separator: [ --- ])

#figure(
  table(columns: 2)[a][b],
  caption: [The table with custom separator.],
)

--- figure-caption-show paged ---
// Test figure.caption element
#show figure.caption: emph

#figure(
  [Not italicized],
  caption: [Italicized],
)

--- figure-caption-where-selector paged ---
// Test figure.caption element for specific figure kinds
#show figure.caption.where(kind: table): underline

#figure(
  [Not a table],
  caption: [Not underlined],
)

#figure(
  table[A table],
  caption: [Underlined],
)

--- figure-par paged ---
// Ensure that a figure body is considered a paragraph.
#show par: highlight

#figure[Text]

#figure(
  [Text],
  caption: [A caption]
)

--- figure-and-caption-show paged ---
// Test creating custom figure and custom caption

#let gap = 0.7em
#show figure.where(kind: "custom"): it => rect(inset: gap, {
  align(center, it.body)
  v(gap, weak: true)
  line(length: 100%)
  v(gap, weak: true)
  align(center, it.caption)
})

#figure(
  [A figure],
  kind: "custom",
  caption: [Hi],
  supplement: [A],
)

#show figure.caption: it => emph[
  #it.body
  (#it.supplement
   #context it.counter.display(it.numbering))
]

#figure(
  [Another figure],
  kind: "custom",
  caption: [Hi],
  supplement: [B],
)

--- figure-caption-position paged empty ---
#set figure.caption(position: top)

--- figure-caption-position-bad eval ---
// Error: 31-38 expected `top` or `bottom`, found horizon
#set figure.caption(position: horizon)

--- figure-localization-fr paged ---
// Test French
#set text(lang: "fr")
#figure(
  circle(),
  caption: [Un cercle.],
)

--- figure-localization-zh paged ---
// Test Chinese
#set text(lang: "zh")
#figure(
  rect(),
  caption: [一个矩形],
)

--- figure-localization-ru paged ---
// Test Russian
#set text(lang: "ru")

#figure(
    polygon.regular(size: 1cm, vertices: 8),
    caption: [Пятиугольник],
)

--- figure-localization-el paged ---
// Test Greek
#set text(lang: "el")
#figure(
  circle(),
  caption: [Ένας κύκλος.],
)

--- issue-2165-figure-caption-panic paged empty ---
#figure.caption[]

--- issue-2328-figure-entry-panic paged ---
// Error: 4-43 footnote entry must have a location
// Hint: 4-43 try using a query or a show rule to customize the footnote instead
HI#footnote.entry(clearance: 2.5em)[There]

--- issue-2530-figure-caption-panic paged ---
#figure(caption: [test])[].caption

--- issue-2821-figure-missing-fields paged empty ---
// Issue #2821: Setting a figure's supplement to none removes the field
#show figure.caption: it => {
  assert(it.has("supplement"))
  assert(it.supplement == none)
}
#figure([], caption: [], supplement: none)

--- issue-3586-figure-caption-separator paged ---
// Test that figure caption separator is synthesized correctly.
#show figure.caption: c => test(c.separator, [#": "])
#figure(table[], caption: [This is a test caption])

--- issue-4966-figure-float-counter paged ---
#let c = context counter(figure.where(kind: image)).display()
#set align(center)

#c

#figure(
  square(c),
  placement: bottom,
  caption: [A]
)

#c

#figure(
  circle(c),
  placement: top,
  caption: [B]
)

#c

--- issue-8823-figure-alt-text html ---
// Test that alt text is output on html figures.
#figure(
  html.frame(square(width: 10pt, fill: red)),
  alt: "A red square",
)
//@@ file tests/suite/model_heading.typ
// Test headings.

--- heading-basic paged ---
// Different number of equals signs.

= Level 1
== Level 2
=== Level 3

// After three, it stops shrinking.
=========== Level 11

--- heading-syntax-at-start paged ---
// Heading vs. no heading.

// Parsed as headings if at start of the context.
/**/ = Level 1
#[== Level 2]
#box[=== Level 3]

// Not at the start of the context.
No = heading

// Escaped.
\= No heading

--- heading-block paged ---
// Blocks can continue the heading.

= #[This
is
multiline.
]

= This
  is not.

--- heading-trailing-whitespace eval ---
// Whether headings contain trailing whitespace with or without comments/labels.
// Labels are special cased to immediately end headings in the parser, but also
// #strike[have unique whitespace behavior] Now their behavior is consistent!

#let join(..xs) = xs.pos().join()
#let head(h) = heading(depth: 1, h)

// No whitespace.
#test(head[h], [= h])
#test(head[h], [= h/**/])
#test(head[h], [= h<a>])
#test(head[h], [= h/**/<b>])

// #strike[Label behaves differently than normal trailing space and comment.]
// Now they behave the same!
#test(join(head[h])[ ], [= h  ])
#test(join(head[h])[ ], [= h  /**/])
#test(join(head[h])[ ], [= h  <c>])

// Combinations.
#test(join(head[h])[ ][ ], [= h  /**/  ])
#test(join(head[h])[ ][ ], [= h  <d>  ])
#test(join(head[h])[ ], [= h  /**/<e>])
#test(join(head[h])[ ], [= h/**/  <f>])

// #strike[The first space attaches, but not the second] Now neither attaches!
#test(join(head(join[h]))[ ][ ], [= h  /**/  <g>])

--- heading-leading-whitespace eval ---
// Test that leading whitespace and comments don't matter.
#test[= h][=        h]
#test[= h][=   /**/  /**/   h]
#test[= h][=   /*
comment spans lines
*/   h]

--- heading-show-where paged ---
// Test styling.
#show heading.where(level: 5): it => block(
  text(font: "Roboto", fill: eastern, it.body + [!])
)

= Heading
===== Heading 🌍
#heading(level: 5)[Heading]

--- heading-offset paged pdftags ---
#set pdf(standard: "ua-1")
// Test setting the starting offset.
#set heading(numbering: "1.1")
#show heading.where(level: 2): set text(blue)
= Level 1

#heading(depth: 1)[We're twins]
#heading(level: 1)[We're twins]

== Real level 2

#set heading(offset: 1)
= Fake level 2
== Fake level 3

--- heading-hanging-indent-auto paged ---
#set heading(numbering: "1.1.a.")
= State of the Art

--- heading-hanging-indent-auto-cj-punct paged ---
#set text(lang: "zh", region: "CN", font: "Noto Serif CJK SC", fallback: false)

// If the numbering starts or ends with a CJ punctuation and its width is
// allowed to be shrunk or stretched, then the heading should still be aligned.
#set heading(numbering: "〔1〕")
= 犹可说也不可说也

--- heading-hanging-indent-auto-center-align paged ---
#set heading(numbering: "1.1.a.")
#show heading: set align(center)
= Center aligned

--- heading-hanging-indent-zero paged ---
#set heading(numbering: "1.1.a.", hanging-indent: 0pt)
= State of the Art

--- heading-hanging-indent-length paged ---
#set heading(numbering: "1.1.a.", hanging-indent: 2em)
= State of the Art In Multi-Line

--- heading-offset-and-level paged ---
// Passing level directly still overrides all other set values
#set heading(numbering: "1.1", offset: 1)
#heading(level: 1)[Still level 1]

--- heading-syntax-edge-cases paged ---
// Edge cases.
#set heading(numbering: "1.")
=
Not in heading
=Nope

--- heading-numbering-hint paged ---
= Heading <intro>

// Error: 19-25 cannot reference heading without numbering
// Hint: 19-25 you can enable heading numbering with `#set heading(numbering: "1.")`
Cannot be used as @intro

--- heading-par paged ---
// Ensure that heading text isn't considered a paragraph.
#show par: highlight
= Heading

--- heading-supplement-panic-trace paged trace ---
// Error: 51-64 panicked with: oops
#set heading(supplement: (h) => if h.depth == 2 { panic("oops") })

= A
// Trace: 1-5 (1) while processing heading element
== B

--- heading-html-basic html ---
// level 1 => h2
// ...
// level 5 => h6
// level 6 => div with role=heading and aria-level=7
//  ...

= Level 1
== Level 2
=== Level 3
==== Level 4
===== Level 5
// Warning: 1-15 heading of level 6 was transformed to <div role="heading" aria-level="7">, which is not supported by all assistive technology
// Hint: 1-15 HTML only supports <h1> to <h6>, not <h7>
// Hint: 1-15 you may want to restructure your document so that it doesn't contain deep headings
====== Level 6
// Warning: 1-16 heading of level 7 was transformed to <div role="heading" aria-level="8">, which is not supported by all assistive technology
// Hint: 1-16 HTML only supports <h1> to <h6>, not <h8>
// Hint: 1-16 you may want to restructure your document so that it doesn't contain deep headings
======= Level 7

--- issue-5719-heading-nested paged ---
// Headings may not be nested like this.
= = A

--- issue-7428-heading-numbering-errors paged ---
// Ensure that the error from the first layout iteration is silenced.
#set heading(numbering: (n, ..nums) => {
  assert(n > 0)
  [#n]
})

= A
//@@ file tests/suite/model_link.typ
// Test hyperlinking.

--- link-basic paged html pdftags ---
#set pdf(standard: "ua-1")
// Link syntax.
https://example.com/

// Link with body.
#link("https://typst.org/")[Some text text text]

// With line break.
This link appears #link("https://google.com/")[in the middle of] a paragraph.

// Certain prefixes are trimmed when using the `link` function.
Contact #link("mailto:hi@typst.app") or
call #link("tel:123") for more information.

--- link-trailing-period paged ---
// Test that the period is trimmed.
#show link: underline
https://a.b.?q=%10#. \
Wahttp://link \
Nohttps:\//link \
Nohttp\://comment

--- link-bracket-balanced paged ---
// Verify that brackets are included in links.
https://[::1]:8080/ \
https://example.com/(paren) \
https://example.com/#(((nested))) \

--- link-bracket-unbalanced-closing paged ---
// Check that unbalanced brackets are not included in links.
#[https://example.com/] \
https://example.com/)

--- link-bracket-unbalanced-opening eval ---
// Verify that opening brackets without closing brackets throw an error.
// Error: 1-22 automatic links cannot contain unbalanced brackets, use the `link` function instead
https://exam(ple.com/

--- link-show paged ---
// Styled with underline and color.
#show link: it => underline(text(fill: rgb("283663"), it))
You could also make the
#link("https://html5zombo.com/")[link look way more typical.]

--- link-transformed paged ---
// Transformed link.
#set page(height: 60pt)
#let mylink = link("https://typst.org/")[LINK]
My cool #box(move(dx: 0.7cm, dy: 0.7cm, rotate(10deg, scale(200%, mylink))))

--- link-on-block paged ---
// Link containing a block.
#link("https://example.com/", block[
  My cool rhino
  #box(move(dx: 10pt, image("/assets/images/rhino.png", width: 1cm)))
])

--- link-url-svg-escape paged ---
// Test that XML metacharacters in a link URL are escaped in the SVG output, so
// that an ordinary query string does not yield malformed SVG.
#link("https://example.com/?a=1&b=2<c>")[#box(width: 5pt, height: 5pt, fill: black)]

--- link-to-page paged ---
// Link to page one.
#link((page: 1, x: 10pt, y: 20pt))[Back to the start]

--- link-to-label paged ---
// Test link to label.
Text <hey>
#link(<hey>)[Go to text.]

--- link-to-label-missing paged ---
// Error: 2-20 label `<hey>` does not exist in the document
#link(<hey>)[Nope.]

--- link-to-label-duplicate paged ---
Text <hey>
Text <hey>
// Error: 2-20 label `<hey>` occurs multiple times in the document
#link(<hey>)[Nope.]

--- link-empty-url eval ---
// Error: 7-9 URL must not be empty
#link("")[Empty]

--- link-empty-block paged ---
#link("https://example.com", block(height: 10pt, width: 100%))

--- issue-758-link-repeat paged ---
#let url = "https://typst.org/"
#let body = [Hello #box(width: 1fr, repeat[.])]

Inline: #link(url, body)

#link(url, block(inset: 4pt, [Block: ] + body))

--- link-html-id-attach html ---
// Tests how IDs and, if necessary, spans, are added to the DOM to support
// links.

#for i in range(1, 10) {
  list.item(link(label("t" + str(i)), [Go]))
}

// Text at start of paragraph
Hi <t1>

// Text at start of paragraph + more text
Hi <t2> there

// Text in the middle of paragraph
See #[it <t4>]

// Text in the middle of paragraph + more text
See #[it <t5>] here

// Text + more elements
See #[a *b*] <t6>

// Element
See *a _b_* <t3>

// Nothing
See #[] <t7>

// Nothing 2
See #metadata(none) <t8>

*Strong* <t9>

--- link-html-label-disambiguation html ---
// Tests automatic ID generation for labelled elements.

#[= A] #label("%") // not reusable => loc-1
= B <1>            // not reusable => loc-3 (loc-2 exists)
= C <loc>          // reusable, unique => loc
= D <loc-2>        // reusable, unique => loc-2
= E <lib>          // reusable, not unique => lib-1
= F <lib>          // reusable, not unique => lib-3 (lib-2 exists)
= G <lib-2>        // reusable, unique => lib-2
= H <hi>           // reusable, unique => hi
= I <hi-2>         // reusable, not unique => hi-2-1
= J <hi-2>         // reusable, not unique => hi-2-2

#context for it in query(heading) {
  list.item(link(it.location(), it.body))
}

--- link-html-id-existing html ---
// Test that linking reuses the existing ID, if any.
#html.div[
  #html.span(id: "this")[This] <other>
]

#link(<other>)[Go]

--- link-html-here html ---
#context link(here())[Go]

--- link-html-nested-empty html ---
#[#metadata(none) <a> #metadata(none) <b> Hi] <c>

#link(<a>)[A] // creates empty span
#link(<b>)[B] // creates second empty span
#link(<c>)[C] // links to #a because the generated span is contained in it

--- link-html-frame html ---
= Frame 1
#html.frame(block(
  stroke: 1pt,
  width: 200pt,
  height: 500pt,
)[
  #place(center, dy: 100pt, stack(
    dir: ltr,
    spacing: 10pt,
    link(<f1>, square(size: 10pt, fill: teal)),
    link(<text>, square(size: 10pt, fill: black)),
    link(<f2>, square(size: 10pt, fill: yellow)),
  ))
  #place(center, dy: 200pt)[
    #square(size: 10pt, fill: teal) <f1>
  ]
])

= Text <text>
#link(<f1>)[Go to teal square]

= Frame 2
#html.frame(block(
  stroke: 1pt,
  width: 200pt,
  height: 500pt,
)[
  #place(center, dy: 100pt)[
    #square(size: 10pt, fill: yellow) <f2>
  ]
])

--- link-html-frame-ref html ---
// Test that reference links work in `html.frame`. Currently, references (and a
// few other elements) do not internally use `LinkElem`s, so they trigger a
// slightly different code path; see `typst-html/src/link.rs`. The text show
// rule is only there to keep the output small.
#set heading(numbering: "1")
#show "Section" + sym.space.nobreak + "1": rect()
#html.frame[@intro]
= Introduction <intro>

--- link-bundle-to-doc bundle ---
// Test directly linking to a different document in the bundle.
#document("index.html")[
  - #link(<index>)[To self] // Should be just `#`
  - #link(<a>)[To A]
  - #link(<b>)[To B]
] <index>
#document("content/a.html")[A] <a>
#document("content/b.pdf")[B] <b>

--- link-bundle-to-asset bundle ---
// Test directly linking to an asset in the bundle.
#document("content/chapter.html", link(<data>)[To data])
#asset("data.json", "[1, 2, 3]") <data>

--- link-bundle-relative bundle ---
// Test relative linking between and into files in the bundle.
#document("index.html",                   link(<b>)[Into B]) <index>
#document("nested/one/b.pdf", [= B <b>] + link(<c>)[Into C])
#document("nested/two/c.svg", [= C <c>] + link(<d>)[Into D])
#document("other/d.html",     [= D <d>] + link(<e>)[Into E])
#document("other/e.html",     [= E <e>] + link(<index>)[Back])

--- link-bundle-label-disambiguation bundle ---
// Tests automatic ID generation for labelled elements in bundles.
// In particular, we test what happens if two elements have the same label
// - in the same document
// - in different documents

// A bit more spacing so that we can better see what's linked to.
#set page(height: auto)
#show heading: set block(spacing: 10cm)

#document("index.html", context {
  query(heading)
    .map(it => list.item(link(it.location(), [To #it.body])))
    .join()
})
#document("x.html")[
  = X1 <x> // Should be `id="x"`
]
#document("a.html")[
  = Open   // Should be `id="loc-1"`
  = X2 <x> // Should be `id="x"`
  = A1 <a> // Should be `id="a-1"`
  = A2 <a> // Should be `id="a-2"`
  = Close  // Should be `id="loc-2"`
]
#document("b.pdf")[
  = Open   // Should be `id="loc-1"`
  = X3 <x> // Should be named destination `x`
  = B1 <b> // Should be named destination `b-1`
  = B2 <b> // Should be named destination `b-2`
  = Close  // Should be `id="loc-2"`
]
#document("c.svg")[
  = X4 <x> // Should be `id="x"`
  = C1 <c> // Should be `id="c-1"`
  = C2 <c> // Should be `id="c-2"`
]

// Not testing PNG since it does not support named destinations.

--- link-bundle-html-frame bundle ---
// Test combination of bundle and frame.
#document("index.html")[
  = Index <index>
  #link(<frame>)[Into frame]
]

#document("folder/a.html")[
  #html.frame[
    #v(10pt)
    = Frame <frame> // Link point should be at `translate(0 10)`
    #link(<index>)[Into index]
  ]
]

--- link-bundle-pdf-internal bundle ---
// During normal PDF export, intradoc links typically use XYZ destinations.
// However, due to the way anchors are auto-assigned for cross-links in bundle
// export, they use `loc-1` style named destinations in bundle export. They work
// just fine, but it's a bit unusual and might be worth changing in the future.
//
// To fix it, during anchor assignment, we'd have to keep track of whether a
// link is actually a cross link and omit it (but only for PDF, SVG still needs
// it).
#document("main.pdf")[
  = A
  #context link(locate(heading))[To heading]
] <pdf>

--- link-bundle-html-a-show-rule bundle ---
// Cross-links currently always use full relative paths. This is not always
// desirable. In the future, there'll likely be better customization for this,
// but in the meantime, this test ensures that it's possible to hack around this
// using an `<a>` show rule.
#show html.elem.where(tag: "a"): it => context {
  let trimmed = it.attrs.href.trim("/index.html", at: end, repeat: false)
  if trimmed.len() < it.attrs.href.len() {
    html.a(href: trimmed, it.body)
  } else {
    it
  }
}

#document("index.html")[
  = Main
  #link(<blog>)[To blog]
] <home>

#document("blog/index.html")[
  #link(<home>)[To home]
] <blog>

--- link-bundle-percent-encoding bundle ---
// Ensure that URIs generated from relative paths in cross-links use percent
// encoding.
#let links = context {
  for doc in query(selector(document).after(<start>)) {
    list.item(link(doc.location(), doc.body))
  }
}

#document("index.pdf", links)
#document("index.html", links)

#metadata(none) <start>
#document("A (1).html")[Space]
#document("A&B.html")[Amp]
#document("%20.html")[Percent]
#document("文.html")[Non-ASCII]
#document("文/b/文.html")[With slash]
//@@ file tests/suite/model_list.typ
// Test bullet lists.

--- list-basic paged ---
_Shopping list_
#list[Apples][Potatoes][Juice]

--- list-nested paged ---
- First level.

  - Second level.
    There are multiple paragraphs.

    - Third level.

    Still the same bullet point.

  - Still level 2.

- At the top.

--- list-content-block paged ---
- Level 1
  - Level #[
2 through content block
]

--- list-top-level-indent paged ---
  - Top-level indent
- is fine.

--- list-indent-specifics paged pdftags ---
#set pdf(standard: "ua-1")
 - A
     - B
   - C
- D

--- list-indent-trivia-nesting eval ---
// Test indent nesting behavior with odd trivia (comments and spaces). The
// comments should _not_ affect the nesting. Only the final column matters.

#let indented = [
- a
 /**/- b
/**/ - c
   /*spanning
     multiple
      lines */ - d
    - e
/**/       - f
/**/  - g
]

#let item = list.item
#let manual = {
  [ ]
  item({
    [a]
    [ ]
    item[b]
    [ ]; [ ]
    item({
      [c]
      [ ]; [ ]
      item[d]
    })
    [ ]
    item({
      [e]
      [ ]; [ ]
      item[f]
      [ ]; [ ]
      item[g]
    })
  })
  [ ]
}

#test(indented, manual)

--- list-indent-bracket-nesting eval ---
// Test list indent nesting behavior when directly at a starting bracket.

#let indented = {
  [- indented
  - less
  ]
  [- indented
   - same
  - then less
   - then same
  ]
  [- indented
    - more
   - then same
  - then less
  ]
}

#let item = list.item
#let manual = {
    {
      item[indented]; [ ]
      item[less]; [ ]
    }
    {
      item[indented]; [ ]
      item[same]; [ ]
      item[then less #{
        item[then same]
      }]; [ ]
    }
    {
      item[indented #{
        item[more]
      }]; [ ]
      item[then same]; [ ]
      item[then less]; [ ]
    }
}

#test(indented, manual)

--- list-tabs paged ---
// This works because tabs are used consistently.
	- A with 1 tab
		- B with 2 tabs

--- list-mixed-tabs-and-spaces paged ---
// This doesn't work because of mixed tabs and spaces.
  - A with 2 spaces
		- B with 2 tabs

--- list-syntax-edge-cases paged ---
// Edge cases.
-
Not in list
-Nope

--- list-marker-align-unaffected paged ---
// Alignment shouldn't affect marker
#set align(horizon)

- ABCDEF\ GHIJKL\ MNOPQR

--- list-marker-dash paged ---
// Test en-dash.
#set list(marker: [--])
- A
- B

--- list-marker-cycle paged ---
// Test that items are cycled.
#set list(marker: ([--], [•]))
- A
  - B
    - C

--- list-marker-closure paged ---
// Test function.
#set list(marker: n => if n == 1 [--] else [•])
- A
- B
  - C
  - D
    - E
- F

--- list-marker-bare-hyphen paged ---
// Test that bare hyphen doesn't lead to cycles and crashes.
#set list(marker: [-])
- Bare hyphen is
- a bad marker

--- list-marker-array-empty eval ---
// Error: 19-21 array must contain at least one marker
#set list(marker: ())

--- list-marker-align-horizontal paged ---
#set list(marker: {
  counter("list").update(n => calc.max(n * 10, 1))
  context counter("list").display()
})

- Item
- Item
- Item

#set list(marker-align: start)
#counter("list").update(0)

- Item
- Item
- Item

--- list-marker-align-vertical paged ---
- #box(fill: teal, inset: 10pt)[a]

#set list(marker-align: top)
- #box(fill: teal, inset: 10pt)[b]

#set list(marker-align: horizon)
- #box(fill: teal, inset: 10pt)[c]

#set list(marker-align: bottom)
- #box(fill: teal, inset: 10pt)[d]

--- list-marker-align-unfolded paged ---
// Marker align option should not be affected by the context.
#[
  #set align(top)
  #set list(marker-align: horizon)

  - #box(fill: teal, inset: 10pt )[]
]

#[
  #set align(horizon)
  - #box(fill: teal, inset: 10pt)[]
]

--- list-marker-align-unfolded-mixed paged ---
// Verify whether overriding vertical alignment causes horizontal alignment to
// be inherited from the context.
#set align(center)
#set list(
  marker-align: top,
  marker: {
    // Artificially cause markers to have a different width.
    counter("b").step()
    context {
      "1" * counter("b").get().first()
    }
  }
)

- abc
- abc
- abc

--- list-marker-align-big-marker paged ---
#set list(
  marker: rect(fill: red, width: 10pt, height: 4em),
  marker-align: bottom,
)


#list[]

- abc

- A\ B\ C\ D\ E\ F

--- list-marker-align-values paged empty ---
// Test valid marker align values (horizontal and vertical)
#set enum(number-align: start)
#set enum(number-align: end)
#set enum(number-align: left)
#set enum(number-align: center)
#set enum(number-align: right)
#set enum(number-align: top)
#set enum(number-align: horizon)
#set enum(number-align: bottom)
#set enum(number-align: start + top)
#set enum(number-align: left + horizon)
#set enum(number-align: center + bottom)

--- list-attached paged ---
// Test basic attached list.
Attached to:
- the bottom
- of the paragraph

Next paragraph.

--- list-attached-above-spacing paged ---
// Test that attached list isn't affected by block spacing.
#show list: set block(above: 100pt)
Hello
- A
World
- B

--- list-non-attached-followed-by-attached paged ---
// Test non-attached list followed by attached list,
// separated by only word.
Hello

- A

World
- B

--- list-tight-non-attached-tight paged ---
// Test non-attached tight list.
#set block(spacing: 15pt)
Hello
- A
World

- B
- C

More.

--- list-wide-cannot-attach paged ---
// Test that wide lists cannot be ...
#set par(spacing: 15pt)
Hello
- A

- B
World

--- list-wide-really-cannot-attach paged ---
// ... even if forced to.
Hello
#list(tight: false)[A][B]
World

--- list-items-context paged ---
#context [+ A]
#context [+ B]
#context [+ C]

--- list-item-styling paged ---
- Hello
#text(red)[- World]
#text(green)[- What up?]

--- list-multi-page paged ---
// Markers should only appear on the first page of each item, and further pages
// should be indented.
#set page(width: auto, height: 4em)

- Abc
  def

  ghi
  jkl

  mno
  pqr
- Other
  other

  other
  other

--- list-colbreak paged ---
- Abc
  def
  #colbreak()
  ghi
  jkl
  #colbreak()
  mno
  pqr

--- list-baseline-table paged ---
- #table(
    inset: 10pt,
    columns: 2,
    [a], [b],
    [c], [d]
  )

- #table(
    inset: 10pt,
    columns: 2,
    stroke: none,
    [a], [b],
    [c], [d]
  )

--- list-baseline-grid paged ---
- #grid(
    inset: 10pt,
    columns: 2,
    [a], [b],
    [c], [d]
  )

--- list-baseline-curve paged ---
#let dy = 15pt
- #curve(
    stroke: 5pt,
    curve.move((0pt,  30pt + dy)),
    curve.line((30pt, 30pt + dy)),
    curve.line((15pt, dy)),
    curve.close()
  )

--- list-baseline-pars paged ---

- #lorem(8)

  #lorem(8)

--- list-baseline-transform paged ---
#set rotate(reflow: true)
#set scale(reflow: true)
#set skew(reflow: true)

- Abc
- #rotate(90deg)[Abc]
- #rotate(180deg)[Abc]
- #scale(30%)[Abc]
- #skew(ax: 30deg)[Abc]
- #skew(ay: 30deg)[Abc]

--- list-baseline-text-with-math paged ---
#set page(width: auto, height: auto)
- Text
  - Text $ "O1" = (7 "O1" + 3 (display((sum_(i = 1)^4 L_i)/4)))/10 $
  - $ "O1" = (7 "O1" + 3 (display((sum_(i = 1)^4 L_i)/4)))/10 $

--- list-baseline-math-fraction paged ---
- $ (7 "O1" + 3 (display((sum_(i = 5)^8 L_i)/4)))/10 $

--- list-baseline-multiline-math paged ---
- $
    sum_(i = 1)^n (x_i)^5 &= 0 \
    y_1 + y_2 &= 10 \
    (a b c)/x^2 &= 5
  $


--- list-baseline-big-marker paged ---
#set list(
  marker: rect(fill: red, width: 10pt, height: 4em),
)


#list[]

- abc

- A\ B\ C\ D\ E\ F

--- list-par paged html ---
// Check whether the contents of list items become paragraphs.
#show par: it => if target() != "html" { highlight(it) } else { it }

#block[
  // No paragraphs.
  - Hello
  - World
]

#block[
  - Hello // Paragraphs

    From
  - World // No paragraph because it's a tight list.
]

#block[
  - Hello // Paragraphs either way

    From

    The

  - World // Paragraph because it's a wide list.
]

--- list-expand-block paged ---
// Lists should shrink to fit their own items inside `auto`-width blocks,
// or expand to the full width of fixed-width containers (/page).
#block[
  - #align(center)[a]
  - bbbb
  - #rect(width: 4em, height: 1em, fill: red)
]

#block(width: 6em)[
  - #align(center)[a]
  - bbbb
  - #rect(width: 4em, height: 1em, fill: red)
]

- #align(center)[a]
- bbbb
- #rect(width: 4em, height: 1em, fill: red)

--- list-expand-auto paged ---
// Lists should shrink to fit their own contents inside auto-width pages.
#set page(width: auto)
- #align(center)[a]
- #rect(width: 4em, height: 1em, fill: red)

longlonglonglonglonglonglong

--- list-big-marker-full-width paged ---
#set page(width: 200pt)
#[
  #set list(indent: 100pt)
  - #lorem(12)
]
#[
  #set list(marker: [AAAAAAAAAAAAAAAAAAAAA])
  - #lorem(12)
]

--- list-big-marker-block paged ---
#set page(width: 200pt)
#block[
  #set list(indent: 100pt)
  - #lorem(12)
]
#block[
  #set list(marker: [AAAAAAAAAAAAAAAAAAAAA])
  - #lorem(12)
]

--- list-negative-indent-full-width paged ---
#set page(width: 200pt)
#[
  #set list(indent: -50pt)
  - #lorem(12)
]
#[
  #set list(marker: box(width: 100%, height: 1em, fill: red))
  - abc
  #set list(indent: -50pt)
  - abc
]

--- list-negative-indent-block paged ---
#set page(width: 200pt)
#block[
  #set list(indent: -50pt)
  - #lorem(12)
]
#block[
  #set list(marker: box(width: 100%, height: 1em, fill: red))
  - abc
  #set list(indent: -50pt)
  - abc
]

--- list-negative-indent-auto paged ---
#set page(width: auto)
#[
  #set list(indent: -50pt)
  - #lorem(20)
]

--- list-vertical-alignment-in-item paged ---
#set page(height: auto)
- a
- #align(bottom)[b]
- c

d

#set page(height: 10em)
- a
- #align(bottom)[b]
- c

d

--- issue-2530-list-item-panic paged ---
// List item (pre-emptive)
#list.item[Hello]

--- issue-1850-list-attach-spacing paged ---
// List attachment should only work with paragraphs, not other blocks.
#set page(width: auto)
#let part = box.with(stroke: 1pt, inset: 3pt)
#{
  part[
    $ x $
    - A
  ]
  part($ x $ + list[A])
  part($ x $ + list[ A ])
  part[
    $ x $

    - A
  ]
  part($ x $ + parbreak() + list[A])
  part($ x $ + parbreak() + parbreak() + list[A])
}

--- issue-5503-list-in-align paged ---
// `align` is block-level and should interrupt a list.
#show list: [List]
- a
- b
#align(right)[- i]
- j

--- issue-5719-list-nested paged ---
// Lists can be immediately nested.
- A
- - B
  - C
- = D
  E

--- issue-6242-tight-list-attach-spacing paged ---
// Nested tight lists should be uniformly spaced when list spacing is set.
#set list(spacing: 1.2em)
- A
  - B
  - C
- C

--- issue-529-list-center-alignment paged ---
#set page(width: 2cm)
- A
- #align(center)[B]
- $ C $

--- issue-1204-list-baseline-alignment paged ---
- A
- $ sum_(i = 1)^n overbrace(x^6, y) $
- #box(baseline: 1cm)[C]
- #v(1cm) D
- #text(48pt)[E]
- #block(inset: 10pt, stroke: red)[Hello world!]
- #rect[Hello world!]

--- issue-8648-enum-gap paged ---
#set page(height: 150pt)
#set enum(number-align: top)

+ #lines(3)
+ #lines(8)
+ #lines(2)
//@@ file tests/suite/model_numbering.typ
// Test integrated numbering patterns.

--- numbering eval ---
#let t(pat: "1", step: 1, ..vals) = {
  let num = 0
  for val in vals.pos() {
    if type(val) == int {
      num = val
    } else {
      test(numbering(pat, num), val)
      num += step
    }
  }
}

// Arabic.
#t(pat: "1", "0", "1", "2", "3", "4", "5", "6", 107, "107", "108")

// Greek.
#t(
  pat: "α",
  "𐆊", "α", "β", "γ", "δ", "ε", "στ", "ζ", "η", "θ", "ι",
  "ια", "ιβ", "ιγ", "ιδ", "ιε", "ιστ", "ιζ", "ιη", "ιθ", "κ",
  241, "σμα",
  999, "ϡϟθ",
  1005, "͵αε",
  1999, "͵αϡϟθ",
  2999, "͵βϡϟθ",
  3000, "͵γ",
  3398, "͵γτϟη",
  4444, "͵δυμδ",
  5683, "͵εχπγ",
  9184, "͵θρπδ",
  9999, "͵θϡϟθ",
)
#t(
  pat: sym.Alpha,
  "𐆊", "Α", "Β", "Γ", "Δ", "Ε", "ΣΤ", "Ζ", "Η", "Θ", "Ι",
  "ΙΑ", "ΙΒ", "ΙΓ", "ΙΔ", "ΙΕ", "ΙΣΤ", "ΙΖ", "ΙΗ", "ΙΘ", "Κ",
  241, "ΣΜΑ",
)

// Symbols.
#t(pat: "*", 1, "*", "†", "‡", "§", "¶", "‖", "**")

// Hebrew.
#t(pat: "א", step: 2, 9, "ט", "יא", "יג", 15, "טו", 16, "טז")

// Chinese.
#t(pat: "一", step: 2, 9, "九", "十一", "十三", "十五", "十七", "十九")
#t(pat: "壹", step: 2, 9, "玖", "拾壹", "拾叁", "拾伍", "拾柒", "拾玖")

// Japanese.
#t(pat: "イ", 1, "イ", "ロ", "ハ", 47, "ス", "イイ", "イロ", "イハ", 2256, "スス", "イイイ")
#t(pat: "い", 1, "い", "ろ", "は", 47, "す", "いい", "いろ", "いは")
#t(pat: "あ", 1, "あ", "い", "う", "え", "お", "か", "き", "く")
#t(pat: "ア", 1, "ア", "イ", "ウ", "エ", "オ", "カ", "キ", "ク")

// Korean.
#t(pat: "가", 1, "가", "나", "다", 47, "다마", "다바", "다사", "다아")
#t(pat: "ㄱ", 1, "ㄱ", "ㄴ", "ㄷ", 47, "ㄷㅁ")

// Arabic Indic.
#t(pat: "\u{0661}", 1475, "١٤٧٥")
#t(pat: "\u{06F1}", 1475, "۱۴۷۵")

// Devanagari.
#t(pat: "\u{0967}", 1, "१")
#t(pat: "\u{0967}", 10, "१०")
#t(pat: "\u{0967}", 123456789, "१२३४५६७८९")

// Bengali.
#t(pat: "\u{09E7}", 1, "১")
#t(pat: "\u{09E7}", 10, "১০")
#t(pat: "\u{09E7}", 123456789, "১২৩৪৫৬৭৮৯")

// Bengali Consonants.
#t(pat: "\u{0995}", 1, "ক")
#t(pat: "\u{0995}", 32, "হ")
#t(pat: "\u{0995}", 32*2 , "কহ")

// Armenian.
#t(pat: "ա", 1, "ա", "բ", "գ", 10, "ժ", 15, "ժե", 24, "իդ", 2025, "սիե")
#t(pat: "Ա", 1, "Ա", "Բ", "Գ", 10, "Ժ", 15, "ԺԵ", 24, "ԻԴ", 2025, "ՍԻԵ")

// Circled number.
#t(pat: "①", 1, "①")
#t(pat: "①", 50, "㊿")

// Double-circled number.
#t(pat: "⓵", 1, "⓵")
#t(pat: "⓵", 10, "⓾")

--- numbering-negative eval ---
// Error: 17-19 number must be at least zero
#numbering("1", -1)

--- numbering-illegal-zero paged ---
#test(numbering("१", 0), "०")
// Warning: 2-19 the numeral system `korean.syllable` cannot represent zero
// Hint: 2-19 this will become a hard error in the future
#numbering("가", 0)

--- numbering-too-high paged ---
// Warning: 2-20 the number 51 is too large to be represented with the `arabic.o` numeral system
// Hint: 2-20 this will become a hard error in the future
#numbering("①", 51)

--- enum-numbering-too-high paged ---
#set enum(numbering: "⓵")
// Warning: 1-9 the number 11 is too large to be represented with the `arabic.oo` numeral system
// Hint: 1-9 this will become a hard error in the future
11. Test

--- page-numbering-too-high paged ---
// Warning: the number 100 is too large to be represented with the `arabic.o` numeral system
// Hint: this will become a hard error in the future
#set page(numbering: "①")
#counter(page).update(100)
Hello

--- page-numbering-too-high-pdf pdf ---
// The page number is not displayed on the page. Instead, it is only computed to
// be embedded in the PDF metadata so the error is triggered in `typst-pdf`
// instead of `typst-layout`. For now, we ignore it and generate the PDF anyway,
// without using the user-provided page numbering.
#set page(numbering: "①", footer: none)
#counter(page).update(100)
Hello

--- footnote-numbering-too-high paged ---
#set footnote(numbering: "①")
#counter(footnote).update(100)
// Warning: 2-12 the number 101 is too large to be represented with the `arabic.o` numeral system
// Hint: 2-12 this will become a hard error in the future
#footnote[]
//@@ file tests/suite/model_outline.typ
--- outline-spacing paged pdftags ---
#set pdf(standard: "ua-1")
#set heading(numbering: "1.a.")
#set outline.entry(fill: none)
#show outline.entry.where(level: 1): set block(above: 1.2em)

#outline()

#show heading: none
= A
== B
== C
= D
== E

--- outline-indent-auto paged ---
#set heading(numbering: "I.i.")
#set page(width: 150pt)
#show heading: none

#context test(outline.indent, auto)
#outline()

= A
== B
== C
== D
=== Title that breaks across lines
= E
== F
=== Aligned

--- outline-indent-auto-mixed-prefix paged ---
#show heading: none
#show outline.entry.where(level: 1): strong

#outline()

#set heading(numbering: "I.i.")
= A
== B
=== Title that breaks
= C
== D
= E
#[
  #set heading(numbering: none)
  = F
  == Numberless title that breaks
  === G
]
= H

--- outline-indent-auto-mixed-prefix-short paged ---
#show heading: none

#outline()

#set heading(numbering: "I.i.")
= A
#set heading(numbering: none)
= B

--- outline-indent-auto-no-prefix paged ---
#show heading: none

#outline()

= A
== B
=== Title that breaks across lines
= C
== D
=== E

--- outline-indent-zero paged ---
#set heading(numbering: "1.a.")
#show heading: none

#outline(indent: 0pt)

= A
== B
=== C
==== Title that breaks across lines
#set heading(numbering: none)
== E
= F

--- outline-indent-fixed paged ---
#set heading(numbering: "1.a.")
#show heading: none

#outline(indent: 1em)

= A
== B
=== C
==== Title that breaks
#set heading(numbering: none)
== E
= F

--- outline-indent-func paged ---
#set heading(numbering: "1.a.")
#show heading: none

#outline(indent: n => (0pt, 1em, 2.5em, 3em).at(n))

= A
== B
=== C
==== Title breaks
#set heading(numbering: none)
== E
= F

--- outline-indent-bad-type paged ---
// Error: 18-34 expected relative length, found dictionary
#outline(indent: n => (a: "dict"))

= Heading

--- outline-indent-function-panic-trace paged trace ---
// Error: 19-26 panicked
#let compute(n) = panic()

// Trace: 22-29 (1) while calling `compute`
#set outline(indent: compute)

// Trace: 2-11 (2) while processing entry element
#outline()

= Heading

--- outline-entry paged ---
#set page(width: 150pt)
#set heading(numbering: "1.")

#show outline.entry.where(level: 1): set block(above: 12pt)
#show outline.entry.where(level: 1): strong

#outline(indent: auto)

#show heading: none
= Introduction
= Background
== History
== State of the Art
= Analysis
== Setup

--- outline-entry-complex paged ---
#set page(width: 150pt, numbering: "I", margin: (bottom: 20pt))
#set heading(numbering: "1.")

#set outline.entry(fill: repeat[--])
#show outline.entry.where(level: 1): it => link(
  it.element.location(),
  it.indented(it.prefix(), {
    emph(it.body())
    [ ]
    text(luma(100), box(width: 1fr, repeat[--·--]))
    [ ]
    it.page()
  })
)

#counter(page).update(3)
#outline()

#show heading: none

= Top heading
== Not top heading
=== Lower heading
=== Lower too
== Also not top

#pagebreak()
#set page(numbering: "1")

= Another top heading
== Middle heading
=== Lower heading

--- outline-entry-inner paged ---
#set heading(numbering: "1.")
#show outline.entry: it => block(it.inner())
#show heading: none

#set outline.entry(fill: repeat[ -- ])
#outline()

= A
= B

--- outline-heading-start-of-page paged ---
#set page(width: 140pt, height: 200pt, margin: (bottom: 20pt), numbering: "1")
#set heading(numbering: "(a/1)")
#show heading.where(level: 1): set text(12pt)
#show heading.where(level: 2): set text(10pt)

#set outline.entry(fill: none)
#outline()

= A
= B
#lines(3)

// This heading is right at the start of the page, so that we can test
// whether the tag migrates properly.
#[
  #set heading(outlined: false)
  == C
]

A

== D
== F
==== G

--- outline-bookmark paged ---
// Ensure that `bookmarked` option doesn't affect the outline
#set heading(numbering: "(I)", bookmarked: false)
#set outline.entry(fill: none)
#show heading: none
#outline()

= A

--- outline-styled-text paged ---
#outline(title: none)

= #text(blue)[He]llo

--- outline-first-line-indent paged ---
#set par(first-line-indent: 1.5em)
#set heading(numbering: "1.1.a.")
#show outline.entry.where(level: 1): strong

#outline()

#show heading: none
= Introduction
= Background
== History
== State of the Art
= Analysis
== Setup

--- outline-bad-element paged ---
// Error: 2-27 cannot outline metadata
#outline(target: metadata)
#metadata("hello")

--- outline-par paged ---
// Ensure that an outline does not produce paragraphs.
#show par: highlight

#outline()

= A
= B
= C

--- outline-html html ---
#set heading(numbering: "1.1.")

#outline()

= A <a>
= B <b>
= C <c>
= D <d>
== E <e>
== F <f>
#heading(outlined: false, [G])
=== H // Should not be nested under F
===== I
#heading(level: 4, bookmarked: false)[J] // `bookmarked` has no effect
= K

--- outline-figure-html html ---
#outline(target: figure, title: [List of Figures])
#figure([A], caption: [The A])
#set text(lang: "fr")
#figure([B], caption: [Le B])
#set text(lang: "de")
#figure([C], caption: figure.caption(separator: " ~ ")[Das C])

--- issue-2048-outline-multiline paged ---
// Without the word joiner between the dots and the page number,
// the page number would be alone in its line.
#set page(width: 125pt)
#set heading(numbering: "1.a.")
#show heading: none

#outline()

= A
== This just fits here

--- issue-2530-outline-entry-panic-text paged ---
// Outline entry (pre-emptive)
// Error: 2-27 cannot outline text
#outline.entry(1, [Hello])

--- issue-2530-outline-entry-panic-heading paged ---
// Outline entry (pre-emptive, improved error)
// Error: 2-34 heading must have a location
// Hint: 2-34 try using a show rule to customize the outline.entry instead
#outline.entry(1, heading[Hello])

--- issue-4476-outline-rtl-title-ending-in-ltr-text paged ---
#set text(lang: "he")
#outline()

#show heading: none
= הוקוס Pocus
= זוהי כותרת שתורגמה על ידי מחשב

--- issue-4859-outline-entry-show-set paged ---
#set heading(numbering: "1.a.")
#show outline.entry.where(level: 1): set outline.entry(fill: none)
#show heading: none

#outline()

= A
== B

--- issue-5176-outline-cjk-title paged ---
#set text(font: "Noto Serif CJK SC")
#show heading: none

#outline(title: none)

= 测
= 很
//@@ file tests/suite/model_par.typ
// Test configuring paragraph properties.

--- par-basic paged pdftags ---
#set pdf(standard: "ua-1")
#set page(width: 250pt, height: 120pt)

But, soft! what light through yonder window breaks? It is the east, and Juliet
is the sun. Arise, fair sun, and kill the envious moon, Who is already sick and
pale with grief, That thou her maid art far more fair than she: Be not her maid,
since she is envious; Her vestal livery is but sick and green And none but fools
do wear it; cast it off. It is my lady, O, it is my love! O, that she knew she
were! She speaks yet she says nothing: what of that? Her eye discourses; I will
answer it.

I am too bold, 'tis not to me she speaks: Two of the fairest stars in all the
heaven, Having some business, do entreat her eyes To twinkle in their spheres
till they return. What if her eyes were there, they in her head? The brightness
of her cheek would shame those stars, As daylight doth a lamp; her eyes in
heaven Would through the airy region stream so bright That birds would sing and
think it were not night. See, how she leans her cheek upon her hand! O, that I
were a glove upon that hand, That I might touch that cheek!

--- par-semantic paged ---
#show par: highlight

I'm a paragraph.

#align(center, table(
  columns: 3,

  // No paragraphs.
  [A],
  block[B],
  block[C *D*],

  // Paragraphs.
  par[E],
  [

    F
  ],
  [
    G

  ],

  // Paragraphs.
  parbreak() + [H],
  [I] + parbreak(),
  parbreak() +  [J] + parbreak(),

  // Paragraphs.
  [K #v(10pt)],
  [#v(10pt) L],
  [#place[] M],

  // Paragraphs.
  [
    N

    O
  ],
  [#par[P]#par[Q]],
  // No paragraphs.
  [#block[R]#block[S]],
))

--- par-semantic-html html ---
= Heading is no paragraph

I'm a paragraph.

#html.elem("div")[I'm not.]

#html.elem("div")[
  We are two.

  So we are paragraphs.
]

--- par-semantic-html-fully-inline html ---
// No paragraphs because everything contained in the divs is fully inline.

#html.div(html.span[A])
#html.div({
  [A ]
  html.span[B]
})
#html.div({
  html.span[A]
  [ B]
})
#html.div({
  html.span[A]
  html.mark[B]
})

#html.div({
  html.label[Text:]
  [ ]
  html.input(type: "text")
})

#html.div[Hello *there*]

--- par-semantic-html-mixed html ---
// Still no paragraphs because the divs contain a mix of inline and neutral
// elements.

#html.div({
  [A]
  html.div[B]
})
#html.div({
  html.span[A]
  html.div[B]
})

#html.div({
  html.div[B]
  [C]
})
#html.div({
  html.div[B]
  html.span[C]
})

#html.div({
  [A]
  html.div[B]
  [C]
})
#html.div({
  html.span[A]
  html.div[B]
  html.span[C]
})

#html.div[
  #html.h2[Heading]
  A
]

--- par-semantic-html-mixed-tags html ---
#html.div({
  html.span()
  html.div()
  // Inline segment starts with a tag.
  metadata(none)
  html.span()
  parbreak()
})

#html.div({
  heading[A]
  // Inline segment ends with a tag.
  html.span()
  metadata(none)
  html.div()
  html.div()
})

#html.div({
  parbreak()
  html.span()
  html.div()
  // Segment consists only of tags.
  metadata(none)
  html.div()
  html.span()
})

--- par-semantic-html-parbreak html ---
// Test that parbreaks force inline elements into paragraphs.

#html.div({
  [Hello ]
  html.strong[there]
  parbreak()
})

#html.div({
  parbreak()
  [A]
})

#html.div[
  Hello *there*

]

#html.div[
  #html.h2[Heading]
  A

  B
]

#html.div({
  parbreak()
  html.span[A]
  html.div[B]
  html.span[C]
  html.div[D]
  html.span[E]
})

#html.div({
  html.span[A]
  html.div[B]
  html.span[C]
  html.div[D]
  html.span[E]
  parbreak()
})

--- par-semantic-html-blocky html ---
// Test that elements with blocky show rules force neighbour inline elements
// into paragraphs.

#html.div({
  heading[Heading]
  [A]
})

#html.div[
  = Heading
  #html.span[A]
]

#html.div[
  A
  #figure[B]
]

#html.div[
  A
  #quote(block: true)[B]
  C
]

#html.div({
  html.span[A]
  html.div[B]
  html.span[C]
  html.div[D]
  html.span[E]
  block[F]
})

#html.div({
  block[A]
  html.span[B]
  html.div[C]
  html.span[D]
  html.div[E]
  html.span[F]
})

--- par-semantic-html-top-level html ---
A few _top-level_ *elements*.

--- par-semantic-html-top-level-typed html ---
A few
#html.em[top-level]
#html.strong[elements].

--- par-semantic-metadata-elements html ---
#html.script("...")
#html.link(rel: "stylesheet", href: "...")

--- par-semantic-tag paged ---
#show par: highlight
#block[
  #metadata(none) <hi1>
  A
  #metadata(none) <hi2>
]

#block(width: 100%, metadata(none) + align(center)[A])
#block(width: 100%, align(center)[A] + metadata(none))

--- par-semantic-align paged ---
#show par: highlight
#show bibliography: none
#set block(width: 100%, stroke: 1pt, inset: 5pt)

#bibliography("/assets/bib/works.bib")

#block[
  #set align(right)
  Hello
]

#block[
  #set align(right)
  Hello
  @netwok
]

#block[
  Hello
  #align(right)[World]
  You
]

#block[
  Hello
  #align(right)[@netwok]
  You
]

--- par-leading-and-spacing paged ---
// Test changing leading and spacing.
#set par(spacing: 1em, leading: 2pt)
But, soft! what light through yonder window breaks?

It is the east, and Juliet is the sun.

--- par-spacing-context paged empty ---
#set par(spacing: 10pt)
#context test(par.spacing, 10pt)

--- par-first-line-indent paged ---
#set par(first-line-indent: 12pt, spacing: 5pt, leading: 5pt)
#show heading: set text(size: 10pt)

The first paragraph has no indent.

But the second one does.

#box(image("/assets/images/tiger.jpg", height: 6pt))
starts a paragraph, also with indent.

#align(center, image("/assets/images/rhino.png", width: 1cm))

= Headings
- And lists.
- Have no indent.

  Except if you have another paragraph in them.

#set text(8pt, lang: "ar", font: ("Noto Sans Arabic", "Libertinus Serif"))
#set par(leading: 8pt)

= Arabic
دع النص يمطر عليك

ثم يصبح النص رطبًا وقابل للطرق ويبدو المستند رائعًا.

--- par-first-line-indent-folding paged empty ---
#let check(expected) = context assert.eq(par.first-line-indent, expected)

// To be intuitive, values from context should never contain `none`.
#check((amount: 0pt, all: false))

#set par(first-line-indent: 2em)
#check((amount: 2em, all: false))

#set par(first-line-indent: (all: true))
#check((amount: 2em, all: true))

/// The following two ways should be the same.
#set par(first-line-indent: 7em)
#check((amount: 7em, all: true))
#set par(first-line-indent: (amount: 1em))
#check((amount: 1em, all: true))

#set par(first-line-indent: (all: false))
#check((amount: 1em, all: false))

#set par(first-line-indent: (amount: 8em, all: true))
#check((amount: 8em, all: true))

--- par-first-line-indent-forbid-all-none eval ---
// Error: 29-53 expected boolean, found none
#set par(first-line-indent: (amount: 2em, all: none))

--- par-first-line-indent-forbid-amount-none eval ---
// Error: 29-54 expected length, found none
#set par(first-line-indent: (amount: none, all: true))

--- par-first-line-indent-columns paged ---
#set par(first-line-indent: (amount: 1em, all: false))

A \ B

C \ D

#colbreak()

// No first line indent after column break
E \ F

G \ H

--- par-first-line-indent-all paged ---
#set par(
  first-line-indent: (amount: 12pt, all: true),
  spacing: 5pt,
  leading: 5pt,
)
#set block(spacing: 1.2em)
#show heading: set text(size: 10pt)

= Heading
All paragraphs are indented.

Even the first.

--- par-first-line-indent-all-list paged ---
#show list.where(tight: false): set list(spacing: 1.2em)
#set par(
  first-line-indent: (amount: 12pt, all: true),
  spacing: 5pt,
  leading: 5pt,
)

- A #parbreak() B #line(length: 100%) C

- D

--- par-first-line-indent-all-enum paged ---
#show enum.where(tight: false): set enum(spacing: 1.2em)
#set par(
  first-line-indent: (amount: 12pt, all: true),
  spacing: 5pt,
  leading: 5pt,
)

+ A #parbreak() B #line(length: 100%) C

+ D

--- par-first-line-indent-all-terms paged pdftags ---
#set pdf(standard: "ua-1")
#show terms.where(tight: false): set terms(spacing: 1.2em)
#set terms(hanging-indent: 10pt)
#set par(
  first-line-indent: (amount: 12pt, all: true),
  spacing: 5pt,
  leading: 5pt,
)

/ Term A: B \ C #parbreak() D #line(length: 100%) E

/ Term F: G

--- par-spacing-and-first-line-indent paged ---
// This is madness.
#set par(first-line-indent: 12pt)
Why would anybody ever ...

... want spacing and indent?

--- par-hanging-indent paged ---
// Test hanging indent.
#set par(hanging-indent: 15pt, justify: true)
#lorem(10)

--- par-hanging-indent-semantic paged ---
#set par(hanging-indent: 15pt)
= I am not affected

I am affected by hanging indent.

--- par-hanging-indent-manual-linebreak paged ---
#set par(hanging-indent: 1em)
Welcome \ here. Does this work well?

--- par-hanging-indent-rtl paged ---
#set par(hanging-indent: 2em)
#set text(dir: rtl, font: ("Libertinus Serif", "Noto Sans Arabic"))
لآن وقد أظلم الليل وبدأت النجوم
تنضخ وجه الطبيعة التي أعْيَتْ من طول ما انبعثت في النهار

--- par-trailing-whitespace paged ---
// Ensure that trailing whitespace layouts as intended.
#box(fill: aqua, " ")

--- par-contains-parbreak paged ---
#par[
  Hello
  // Warning: 4-14 parbreak may not occur inside of a paragraph and was ignored
  #parbreak()
  World
]

--- par-contains-block paged ---
#par[
  Hello
  // Warning: 4-11 block may not occur inside of a paragraph and was ignored
  #block[]
  World
]

--- par-empty-metadata paged empty ---
// Check that metadata still works in a zero length paragraph.
#block(height: 0pt)[#""#metadata(false)<hi>]
#context test(query(<hi>).first().value, false)

--- par-metadata-after-trimmed-space paged ---
// Ensure that metadata doesn't prevent trailing spaces from being trimmed.
#set par(justify: true, linebreaks: "simple")
#set text(hyphenate: false)
Lorem ipsum dolor #metadata(none) nonumy eirmod tempor.

--- par-show-children paged ---
// Variant 1: Prevent recursion by checking the children.
#let p = counter("p")
#let step = p.step()
#let nr = context p.display()
#show par: it => {
  if it.body.at("children", default: ()).at(0, default: none) == step {
    return it
  }
  par(step + [§#nr ] + it.body)
}

= A

B

C #parbreak() D

#block[E]

#block[F #parbreak() G]

--- par-show-styles paged ---
// Variant 2: Prevent recursion by observing a style.
#let revoke = metadata("revoke")
#show par: it => {
  if bibliography.title == revoke { return it }
  set bibliography(title: revoke)
  let p = counter("p")
  par[#p.step()§#context p.display() #it.body]
}

= A

B

C

--- par-explicit-trim-space paged ---
A

#par[ B ]

--- issue-4278-par-trim-before-equation paged ---
#set par(justify: true)
#lorem(6) aa $a = c + b$

--- issue-4938-par-bad-ratio paged ---
#set par(justify: true)
#box($k in NN_0$)

--- issue-4770-par-tag-at-start paged empty ---
#h(0pt) #box[] <a>

#context test(query(<a>).len(), 1)

--- issue-5831-par-constructor-args paged ---
// Make sure that all arguments are also respected in the constructor.
A
#par(
  leading: 2pt,
  spacing: 20pt,
  justify: true,
  linebreaks: "simple",
  first-line-indent: (amount: 1em, all: true),
  hanging-indent: 5pt,
)[
  The par function has a constructor and justification.
]

--- show-par-set-block-hint paged empty ---
// Warning: 2-36 `show par: set block(spacing: ..)` has no effect anymore
// Hint: 2-36 this is specific to paragraphs as they are not considered blocks anymore
// Hint: 2-36 write `set par(spacing: ..)` instead
#show par: set block(spacing: 12pt)
//@@ file tests/suite/model_quote.typ
// Test the quote element.

--- quote-dir-author-pos paged pdftags ---
#set pdf(standard: "ua-1")
// Text direction affects author positioning
#set text(font: ("Libertinus Serif", "Noto Sans Arabic"))
And I quote: #quote(attribution: [René Descartes])[cogito, ergo sum].

#set text(lang: "ar")
#quote(attribution: [عالم])[مرحبًا]

--- quote-dir-align paged pdftags ---
#set pdf(standard: "ua-1")
// Text direction affects block alignment
#set text(font: ("Libertinus Serif", "Noto Sans Arabic"))
#set quote(block: true)
#quote(attribution: [René Descartes])[cogito, ergo sum]

#set text(lang: "ar")
#quote(attribution: [عالم])[مرحبًا]

--- quote-block-spacing paged ---
// Spacing with other blocks
#set quote(block: true)
#set text(8pt)

#lines(3)
#quote(lines(3))
#lines(3)

--- quote-inline paged ---
// Inline citation
#set text(8pt)
#quote(attribution: <tolkien54>)[In a hole in the ground there lived a hobbit.]

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- quote-cite-heading-error paged ---
#set heading(numbering: "1.1")

= Heading <h>
// Error: 2-48 citation key `h` is not present in the bibliography
#quote(attribution: <h>, block: true, lorem(5))

#bibliography(bytes(""))

--- quote-cite-format-label-or-numeric paged ---
// Citation-format: label or numeric
#set text(8pt)
#set quote(block: true)
#quote(attribution: <tolkien54>)[In a hole in the ground there lived a hobbit.]

#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "ieee")

--- quote-cite-format-note paged ---
// Citation-format: note
#set text(8pt)
#set quote(block: true)
#quote(attribution: <tolkien54>)[In a hole in the ground there lived a hobbit.]

#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "chicago-shortened-notes")

--- quote-cite-format-author-date paged ---
// Citation-format: author-date or author
#set text(8pt)
#set quote(block: true)
#quote(attribution: <tolkien54>)[In a hole in the ground there lived a hobbit.]

#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "apa")

--- quote-nesting paged ---
// Test quote selection.
#set page(width: auto)
#set text(lang: "en")
=== EN
#quote[An apostroph'] \
#quote[A #quote[nested] quote] \
#quote[A #quote[very #quote[nested]] quote]

#set text(lang: "de")
=== DE
#quote[Satz mit Apostroph'] \
#quote[Satz mit #quote[Zitat]] \
#quote[A #quote[very #quote[nested]] quote]

#set smartquote(alternative: true)
=== DE Alternative
#quote[Satz mit Apostroph'] \
#quote[Satz mit #quote[Zitat]] \
#quote[A #quote[very #quote[nested]] quote]

--- quote-nesting-custom paged ---
// With custom quotes.
#set smartquote(quotes: (single: ("<", ">"), double: ("(", ")")))
#quote[A #quote[nested] quote]

--- quote-plato html ---
#set quote(block: true)

#quote(attribution: [Plato])[
  ... ἔοικα γοῦν τούτου γε σμικρῷ τινι αὐτῷ τούτῳ σοφώτερος εἶναι, ὅτι
  ἃ μὴ οἶδα οὐδὲ οἴομαι εἰδέναι.
]
#quote(attribution: [from the Henry Cary literal translation of 1897])[
  ... I seem, then, in just this little thing to be wiser than this man at
  any rate, that what I do not know I do not think I know either.
]

--- quote-nesting-html html ---
When you said that #quote[he surely meant that #quote[she intended to say #quote[I'm sorry]]], I was quite confused.

--- quote-attribution-link html ---
#quote(
  block: true,
  attribution: link("https://typst.app/home")[typst.com]
)[
  Compose papers faster
]

--- quote-attribution-html html ---
A
#quote(block: true, attribution: [C])[B]
D

--- quote-par paged ---
// Ensure that an inline quote is part of a paragraph, but a block quote
// does not result in paragraphs.
#show par: highlight

An inline #quote[quote.]

#quote(block: true, attribution: [The Test Author])[
  A block-level quote.
]

--- issue-5536-quote-inline-quotes-false paged ---
Lorem #quote(block: false, quotes: false)[dolor].
//@@ file tests/suite/model_ref.typ
// Test references.

--- ref-basic paged html ---
#set heading(numbering: "1.")

= Introduction <intro>
See @setup.

== Setup <setup>
As seen in @intro, we proceed.

--- ref-label-missing paged ---
// Error: 1-5 label `<foo>` does not exist in the document
@foo

--- ref-label-duplicate paged ---
= First <foo>
= Second <foo>

// Error: 1-5 label `<foo>` occurs multiple times in the document
@foo

--- ref-supplements paged ---
#set heading(numbering: "1.", supplement: [Chapter])
#set math.equation(numbering: "(1)", supplement: [Eq.])

= Intro
#figure(
  image("/assets/images/cylinder.svg", height: 1cm),
  caption: [A cylinder.],
  supplement: "Fig",
) <fig1>

#figure(
  image("/assets/images/tiger.jpg", height: 1cm),
  caption: [A tiger.],
  supplement: "Tig",
) <fig2>

$ A = 1 $ <eq1>

#set math.equation(supplement: none)
$ A = 1 $ <eq2>

@fig1, @fig2, @eq1, (@eq2)

#set ref(supplement: none)
@fig1, @fig2, @eq1, @eq2

--- ref-ambiguous paged ---
// Test ambiguous reference.
= Introduction <arrgh>

// Error: 1-7 label `<arrgh>` occurs both in the document and a bibliography
// Hint: 1-7 change either the heading's label or the bibliography key to resolve the ambiguity
@arrgh
#bibliography("/assets/bib/works.bib")

--- ref-form-page paged ---
#set page(numbering: "1")

Text <text> is on #ref(<text>, form: "page").
See #ref(<setup>, form: "page").

#set page(supplement: [p.])

== Setup <setup>
Text seen on #ref(<text>, form: "page").
Text seen on #ref(<text>, form: "page", supplement: "Page").

--- ref-form-page-unambiguous paged ---
// Test that page reference is not ambiguous.
#set page(numbering: "1")

= Introduction <arrgh>

#ref(<arrgh>, form: "page")
#bibliography("/assets/bib/works.bib")

--- ref-form-page-bibliography paged ---
// Error: 2-28 label `<quark>` does not exist in the document
#ref(<quark>, form: "page")
#bibliography("/assets/bib/works.bib")

--- issue-4536-non-whitespace-before-ref paged empty ---
// Test reference with non-whitespace before it.
#figure[] <1>
#test([(#ref(<1>))], [(@1)])

--- ref-to-empty-label-not-possible paged ---
// @ without any following label should just produce the symbol in the output
// and not produce a reference to a label with an empty name.
@

--- ref-function-empty-label eval ---
// using ref() should also not be possible
// Error: 6-7 unexpected less-than operator
// Error: 7-8 unexpected greater-than operator
#ref(<>)
//@@ file tests/suite/model_terms.typ
// Test term list.

--- terms-constructor paged pdftags ---
#set pdf(standard: "ua-1")
// Test with constructor.
#terms(
  terms.item([One], [First]),
  terms.item([Two], [Second]),
)

--- terms-built-in-loop paged ---
// Test joining.
#for word in lorem(4).split().map(s => s.trim(".")) [
  / #word: Latin stuff.
]

--- terms-multiline paged ---
// Test multiline.
#set text(8pt)

/ Fruit: A tasty, edible thing.
/ Veggie:
  An important energy source
  for vegetarians.

  And healthy!

--- terms-style-change-interrupted paged ---
// Test style change.
#set text(8pt)
/ First list: #lorem(6)

#set terms(hanging-indent: 30pt)
/ Second list: #lorem(5)

--- terms-rtl paged ---
// Test RTL.
#set text(8pt, dir: rtl)

/ פרי: דבר טעים, אכיל. ומקור אנרגיה חשוב לצמחונים.

--- terms-grid paged ---
// Test grid like show rule.
#show terms: it => table(
  columns: 2,
  inset: 3pt,
  ..it.children.map(v => (emph(v.term), v.description)).flatten(),
)

/ A: One letter
/ BB: Two letters
/ CCC: Three letters

--- terms-syntax-edge-cases paged ---
/ Term:
Not in list
/Nope

--- terms-missing-colon eval ---
// Error: 8 expected colon
/ Hello

--- terms-par paged html ---
// Check whether the contents of term list items become paragraphs.
#show par: it => if target() != "html" { highlight(it) } else { it }

// No paragraphs.
#block[
  / Hello: A
  / World: B
]

#block[
  / Hello: A // Paragraphs

    From
  / World: B // No paragraphs because it's a tight term list.
]

#block[
  / Hello: A // Paragraphs

    From

    The

  / World: B // Paragraph because it's a wide term list.
]

--- issue-1050-terms-indent paged ---
#set page(width: 110pt)
#set par(first-line-indent: 0.5cm)

- #lorem(5)
- #lorem(5)

+ #lorem(5)
+ #lorem(5)

/ S: #lorem(5)
/ XXXL: #lorem(5)

--- issue-2530-term-item-panic paged ---
// Term item (pre-emptive)
#terms.item[Hello][World!]

--- issue-5503-terms-in-align paged ---
// `align` is block-level and should interrupt a `terms`.
#show terms: [Terms]
/ a: a
#align(right)[/ i: i]
/ j: j

--- issue-5719-terms-nested paged ---
// Term lists can be immediately nested.
/ Term A: 1
/ Term B: / Term C: 2
          / Term D: 3
//@@ file tests/suite/model_title.typ
// Test title element.

--- title paged html pdftags ---
#set pdf(standard: "ua-1")
#set document(title: [My title])
#title()
= A level one heading

--- title-with-body paged html ---
#set document(title: [My title])
#title[My display title]

--- title-with-body-auto paged ---
#set document(title: [My title])
#title(auto)

--- title-show-set paged ---
#show title: set text(blue)
#title[A blue title]

--- title-unset paged ---
// Error: 2-9 document title was not set
// Hint: 2-9 set the title with `set document(title: [...])`
// Hint: 2-9 or provide an explicit body with `title[..]`
#title()
//@@ file tests/suite/pdf_attach.typ
// Test file attachments. The tests here so far are unsatisfactory because we
// have no PDF testing infrastructure. That should be improved in the future.

--- pdf-attach paged empty ---
#pdf.attach("/assets/text/hello.txt")
#pdf.attach(
  "/assets/data/details.toml",
  relationship: "supplement",
  mime-type: "application/toml",
  description: "Information about a secret project",
)

--- pdf-attach-bytes paged empty ---
#pdf.attach("hello.txt", read("/assets/text/hello.txt", encoding: none))
#pdf.attach(
  "a_file_name.txt",
  read("/assets/text/hello.txt", encoding: none),
  relationship: "supplement",
  mime-type: "text/plain",
  description: "A description",
)

--- pdf-attach-zero-bytes paged empty ---
#pdf.attach("file", bytes(()))

--- pdf-attach-invalid-relationship eval ---
#pdf.attach(
  "/assets/text/hello.txt",
  // Error: 17-23 expected "source", "data", "alternative", "supplement", or none
  relationship: "test",
  mime-type: "text/plain",
  description: "A test file",
)

--- pdf-attach-invalid-data eval ---
// Error: 39-46 expected bytes, found string
#pdf.attach("/assets/text/hello.txt", "hello")
//@@ file tests/suite/pdf_validation.typ
--- pdf-validation-multiple-standards pdf ---
#set pdf(standard: ("a-2a", "ua-1"))
#set document(date: datetime(year: 1970, month: 1, day: 1))
Hello

--- pdf-validation-version-incompatible eval ---
// Error: 20-34 PDF 1.4 is not compatible with PDF/A-4
// Hint: 20-34 PDF/A-4 requires version PDF 2.0
#set pdf(standard: ("1.4", "a-4"))
Hello

--- pdf-validation-standards-incompatible eval ---
// Error: 20-35 PDF/A-4 and PDF/UA-1 are mutually incompatible because they do not have any overlapping PDF versions
// Hint: 20-35 PDF/A-4 requires version PDF 2.0
// Hint: 20-35 PDF/UA-1 requires a version between PDF 1.4 and PDF 1.7
#set pdf(standard: ("ua-1", "a-4"))
Hello

--- pdf-validation-min-feature-version-unsatisfiable pdf ---
#set pdf(standard: ("ua-1", "a-1a"))
#set document(date: datetime(year: 1970, month: 1, day: 1))
#table(
  // Error: 3-37 PDF/UA-1 error: table header cell cannot be accessibly tagged in PDF 1.4 files
  // Hint: 3-37 PDF version must be at least PDF 1.5 to satisfy PDF/UA-1
  // Hint: 3-37 remove or replace the PDF/A-1a standard, as it prevents PDF versions beyond PDF 1.4
  pdf.header-cell(scope: "row")[brr]
)

--- pdf-validation-missing-feature-single-version pdf ---
#set pdf(standard: ("1.4", "ua-1"))
// Error: PDF/UA-1 error: headers and footers cannot be made accessible in PDF 1.4 files
// Hint: set the version to PDF 1.7
#set page(header: [Hi])

--- pdf-validation-missing-feature-multiple-versions pdf ---
#set pdf(standard: ("1.4", "ua-1"))
// Error: PDF/UA-1 error: links and other annotations cannot be navigated accessibly in PDF 1.4 files
// Hint: select a version between PDF 1.5 and PDF 1.7
https://typst.app/

--- pdf-validation-page-range-and-tagged pdf ---
// Error: 2-33 cannot enable tagged PDF and export a page range
#set pdf(pages: 1, tagged: true)

--- pdf-validation-tofu pdf ---
#set pdf(standard: "ua-1")
// Error: 1-2 PDF/UA-1 error: the text `"ግ"` could not be displayed with font `"Libertinus Serif"`
// Hint: 1-2 try using a different font
ግ

--- pdf-validation-tofu-in-svg pdf ---
#set pdf(standard: "ua-1")
// A spanless error without a font name is kinda bad, but this used to be a
// crash, so it's already an improvement.

// Error: PDF/UA-1 error: the text `"ግ"` could not be displayed with a font
// Hint: try using a different font
#image(bytes(
  ```
  <?xml version="1.0" encoding="utf-8"?>
  <svg width="10" height="10" xmlns="http://www.w3.org/2000/svg">
    <text>ግ</text>
  </svg>
  ```.text
), alt: "Geʽez letter")

--- pdf-validation-bundle bundle ---
#set pdf(standard: "ua-1")
#document(
  "hi.pdf",
  // Error: 13-28 PDF/UA-1 error: cannot combine underline, overline, or strike
  underline(overline[Hello]),
)

--- issue-8383-headers-pdf-ua pdf ---
#set pdf(standard: "ua-1")
#set page(header: [A])
//@@ file tests/suite/pdftags_break.typ
--- break-tags-nested-parbreak pdftags ---
#let target = "tel:123"

Start of the first paragraph #link(target)[
  #quote[
    Part of the first paragraph.

    Start of the second paragraph
  ]
] Part of the second paragraph.

--- break-tags-nested-parbreak-with-nested-groups pdftags ---
#let target = "tel:123"

Start of the first paragraph #link(target)[
  `group before`
  #quote[
    `group before`
    Part of the first paragraph.

    Start of the second paragraph
  ]
] Part of the second paragraph.

--- issue-7020-break-tags pdftags ---
Foo #quote($$ + parbreak())

--- issue-7257-break-tags-show-par-none pdftags ---
#show par: none
#show heading: v(0pt) + [A]
// Error: 3-6 internal error: tags weren't properly closed (occurred at crates/typst-pdf/src/tags/tree/build.rs:191:9)
// Hint: 3-6 please report this as a bug
#[= A]B
//@@ file tests/suite/pdftags_deco.typ
--- deco-tags-underline pdftags ---
#set pdf(standard: "ua-1")
#show: underline.with(stroke: red)

// The content in this paragraph is grouped into one span tag with the
// corresponding text attributes.
red underlined text
red underlined text

red underlined text

--- deco-tags-different-color pdftags ---
#set pdf(standard: "ua-1")
#show: underline.with(stroke: red)
red underlined text
#show: underline.with(stroke: blue)
blue underlined text

--- deco-tags-different-stroke-thickness pdftags ---
#set pdf(standard: "ua-1")
#show: underline.with(stroke: 2pt)
thick underlined
#show: underline.with(stroke: 1pt)
thin underlined

--- deco-tags-different-type pdftags ---
#set pdf(standard: "ua-1")
#underline[underlined]\
#overline[overlined]\
#strike[striked]\

--- deco-tags-multiple-styles pdftags ---
#set pdf(standard: "ua-1")
#show: underline
// Error: 2-16 PDF/UA-1 error: cannot combine underline, overline, or strike
#show: overline
text with a bunch of lines

--- deco-tags-highlight-basic pdftags ---
#set pdf(standard: "ua-1")
A #highlight[highlighted] alksjdflk asdjlkfj alskdj word.

--- deco-tags-subscript-basic pdftags ---
#set pdf(standard: "ua-1")
CO#sub[2] emissions.
A2#sub[hex]

--- deco-tags-superscript-basic pdftags ---
#set pdf(standard: "ua-1")
CI#super[-] has a negative charge.

--- deco-tags-script-custom-baseline pdftags ---
#set pdf(standard: "ua-1")
// NOTE: the baseline shift values attribute is inverted.
#set sub(baseline: 2.5pt)
#set super(baseline: -9.5pt)
#sub[sub]
#super[super]

--- deco-tags-emph-basic pdftags ---
#set pdf(standard: "ua-1")
Cats are _cute_ animals.

--- deco-tags-strong-basic pdftags ---
#set pdf(standard: "ua-1")
This *HERE* is important!

--- deco-tags-strong-and-em pdftags ---
#set pdf(standard: "ua-1")
_*strong and emph*_

--- deco-tags-strong-em-and-more-attrs pdftags ---
#set pdf(standard: "ua-1")
#underline(stroke: green)[_*strong and emph*_]
//@@ file tests/suite/pdftags_disable.typ
--- disable-tags-artifact pdftags ---
#set pdf(standard: "ua-1")
= Heading 1
#pdf.artifact[
  #table(
    columns: 2,
    [a], [b],
    [c], [d],
  )
]

= Heading 2

--- disable-tags-tiling pdftags ---
#set pdf(standard: "ua-1")
= Rectangle

#let pat = tiling(size: (20pt, 20pt))[
  - a
  - b
    - c
]
#rect(fill: pat)

--- disable-tags-hide pdftags ---
#set pdf(standard: "ua-1")
= Hidden

#hide[
  - a
  - b
    - c
]

--- disable-tags-partially-hidden-list pdftags ---
#set pdf(standard: "ua-1")
// FIXME(accessibility): In realization, tags inside of list groupings aren't
// handled. Thus if the head of the list is visible, all tags of list items
// will be emitted before (outside) the hide element. And if the head is not
// visible, all tags of list items will be emitted inside the hide element.
= Tail hidden
- a
#hide[
- b
  - c
]

= Head hidden
#hide[
- a
]
- b
  - c

--- disable-tags-broken-paragraph-hide pdftags ---
#set pdf(standard: "ua-1")
This is the #hide[ first paragraph.

And this is the ] second paragraph.

--- disable-tags-broken-paragraph-artifact pdftags ---
#set pdf(standard: "ua-1")
This is the #pdf.artifact[ first paragraph.

And this is the ] second paragraph.

--- background-artifact-1-7 pdf ---
#set pdf(standard: "1.7")
#set page(fill: eastern)
#rect()
#pdf.artifact(kind: "background", circle())

--- background-artifact-2-0 pdf ---
#set pdf(standard: "2.0")
#set page(fill: eastern)
#rect()
#pdf.artifact(kind: "background", circle())
//@@ file tests/suite/pdftags_figure.typ
--- figure-tags-image-basic pdftags ---
#set pdf(standard: "ua-1")
// The image is automatically wrapped in a figure tag.
#image(alt: "A tiger", "/assets/images/tiger.jpg")

--- figure-tags-image-figure-with-caption pdftags ---
#set pdf(standard: "ua-1")
#figure(
  // The image doesn't create a duplicate figure tag
  // and the alt description is used in the outer figure.
  image(alt: "A tiger", "/assets/images/tiger.jpg"),
  // The caption is stored outside of the figure tag
  // grouped in a nonstruct.
  caption: [Some caption]
)

--- figure-tags-inline-equation-with-caption pdftags ---
#set pdf(standard: "ua-1")
#figure(
  math.equation(
    alt: "The Pythagorean theorem: a squared plus b squared is c squared",
    $a^2 + b^2 = c^2$,
  ),
  caption: [Some caption]
)

--- figure-tags-block-equation-with-caption pdftags ---
#set pdf(standard: "ua-1")
#figure(
  // The alt description is used in the outer figure.
  math.equation(
    block: true,
    alt: "The Pythagorean theorem: a squared plus b squared is c squared",
    $
      a^2 + b^2 = c^2
    $,
  ),
  caption: [Some caption]
)

--- figure-tags-with-alt-flatten-content-basic pdftags ---
#set pdf(standard: "ua-1")
// The inner tags are flattened
#figure(alt: "alt text")[
  $a^2 + b^2 = c^2$

  $sum_(i=1)^n(i)$
]

--- figure-tags-with-alt-flatten-content-complex pdftags ---
#set pdf(standard: "ua-1")
#figure(alt: "alt text")[
  #table(
    columns: 2,
    // The link tag needs to be retained
    link("https://github.com/typst/typst")[
      #image("/assets/images/tiger.jpg")
    ],
    image("/assets/images/tiger.jpg"),
    [Some more text],
  )
]

--- figure-tags-alt-with-different-lang pdftags ---
#set pdf(standard: "ua-1")
#set text(lang: "de")
Ein Paragraph.

#set text(lang: "en", region: "uk")
#figure(image(alt: "A tiger", "/assets/images/tiger.jpg"))

--- figure-tags-listing pdftags ---
#set pdf(standard: "ua-1")
#figure[
  ```rs
  fn main() {
      println!("Hello Typst!");
  }
  ```
]

--- figure-tags-only-marked-content-missing-alt pdftags ---
#set pdf(standard: "ua-1")
// Error: 1:2-3:2 PDF/UA-1 error: missing alt text
// Hint: 1:2-3:2 make sure your images and equations have alt text
#figure[
  #rect(fill: red)
]

--- figure-tags-only-marked-content pdftags ---
#figure[
  #rect(fill: red)
]

--- figure-tags-only-marked-content-with-alt pdftags ---
#figure(
  rect(fill: red),
  alt: "A red rectangle",
)

--- figure-tags-equation-only-marked-content-with-alt pdftags ---
#math.equation(
  rect(fill: red),
  alt: "A red rectangle",
)

--- figure-tags-equation-only-marked-content-with-alt-ua-1 pdftags ---
#set pdf(standard: "ua-1")
#math.equation(
  rect(fill: red),
  alt: "A red rectangle",
)

--- figure-tags-additional-caption-inside-body pdftags ---
#figure(caption: [The real caption])[
  #image(alt: "A tiger", "/assets/images/tiger.jpg"),
  #figure.caption[Additional caption]
]

--- figure-tags-additional-caption-inside-table pdftags ---
#set pdf(standard: "ua-1")
// Error: 19-35 PDF/UA-1 error: invalid table (Table) structure
// Hint: 19-35 table (Table) may not contain multiple caption (Caption) tags
// Hint: 19-35 avoid manually calling `figure.caption`
#figure(caption: [The real caption])[
  #table(
    columns: 2,
    [A], [B],
    [C], [D],
  )
// Error: 4-38 PDF/UA-1 error: invalid table (Table) structure
// Hint: 4-38 table (Table) may not contain multiple caption (Caption) tags
// Hint: 4-38 avoid manually calling `figure.caption`
  #figure.caption[Additional caption]
]

--- figure-tags-bbox-of-square-with-stroke pdftags ---
#set pdf(standard: "ua-1")
#figure(
  alt: "A square with a red stroke",
  square(size: 60pt, stroke: 10pt + red)
)

--- issue-8703-single-page-figure-without-bbox pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 100pt)
#set page(header: [hi])

#figure(
  alt: "A rect",
  rect(width: 80pt, height: 80pt),
)

Hi
//@@ file tests/suite/pdftags_footnote.typ
--- footnote-tags-basic pdftags ---
#set pdf(standard: "ua-1")
Footnote #footnote[Hi] in text.

--- footnote-tags-different-lang pdftags ---
#set pdf(standard: "ua-1")
Footnote #footnote[
  // The footnote number is still in English ("en"), so the link tag
  // holding the number should specify its language to be English, so
  // as to override the parent tag's language, which is German ("de").
  #set text(lang: "de")
  Hallo
] in text.

--- footnote-tags-ref-to-other-footnote pdftags ---
#set pdf(standard: "ua-1")
This #footnote[content]<note> and #footnote(<note>).
//@@ file tests/suite/pdftags_grid.typ
--- grid-tags-rowspan pdftags ---
#set pdf(standard: "ua-1")
#grid(
  columns: 4,
  stroke: 1pt,
  rows: 3,
  // the code cell should come first in the reading order
  grid.cell(rowspan: 3)[`code`], [b], [c], [d],
  // the underline cell should come second to last
  [b], grid.cell(x: 2, y: 1, colspan: 2, rowspan: 2, underline[text]),
  [b],
)

--- grid-tags-cell-breaking pdftags ---
#set pdf(standard: "ua-1")
// The second paragraph contains marked content from page 1 and 2
#set page(width: 5cm, height: 3cm)
#grid(
  columns: 2,
  row-gutter: 8pt,
  [Lorem ipsum dolor sit amet.

  Aenean commodo ligula eget dolor. Aenean massa. Penatibus et magnis.],
  [Text that is rather short],
  [Fireflies],
  [Critical],
  [Decorum],
  [Rampage],
)

--- grid-tags-rowspan-split-3 pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 5em)
#grid(
  columns: 2,
  fill: red,
  inset: 0pt,
  grid.cell(fill: orange, rowspan: 10, place(bottom)[*Z*] + [x\ ] * 10 + place(bottom)[*ZZ*]),
  ..([y],) * 10,
  [a], [b],
)

--- grid-tags-internal-grid-layout-breaking-bibliography pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 140pt)

#cite(label("DBLP:books/lib/Knuth86a"))

#bibliography(
  "/assets/bib/works.bib",
  style: "ieee",
)
//@@ file tests/suite/pdftags_heading.typ
--- heading-tags-basic pdftags ---
#set pdf(standard: "ua-1")
= Level 1
== Level 2
=== Level 3

--- heading-tags-first-is-not-level-1 pdftags ---
#set pdf(standard: "ua-1")
// Error: 1-11 PDF/UA-1 error: the first heading must be of level 1
== Level 2

--- heading-tags-non-consecutive-levels pdftags ---
#set pdf(standard: "ua-1")
= Level 1
// Error: 1-12 PDF/UA-1 error: skipped from heading level 1 to 3
// Hint: 1-12 heading levels must be consecutive
=== Level 3

--- heading-tags-complex pdftags ---
#set pdf(standard: "ua-1")
= Level 1
== Level 2
=== Level 3
=== Level 3
== Level 2
=== Level 3
==== Level 4
== Level 2
=== Level 3
=== Level 3
=== Level 3
= Level 1
== Level 2

--- heading-tags-empty pdftags ---
#set pdf(standard: "ua-1")
// Error: 1-2 PDF/UA-1 error: heading title is empty
=

--- heading-tags-context-body pdftags ---
#set pdf(standard: "ua-1")
// Error: 2-32 PDF/UA-1 error: heading title could not be determined
// Hint: 2-32 this seems to be caused by a context expression within the heading
// Hint: 2-32 consider wrapping the entire heading in a context expression instead
#heading(context [Hello there])
//@@ file tests/suite/pdftags_lang.typ
--- lang-tags-pars-basic pdftags ---
#set pdf(standard: "ua-1")
#set text(lang: "uk")
Par 1.

#set text(lang: "sr")
Par 2.

#set text(lang: "be")
Par 3.

--- lang-tags-propagation pdftags ---
#set pdf(standard: "ua-1")
#set text(lang: "nl")
A paragraph.

// The list bullets are in spanish :)
#set text(lang: "es", region: "co")
- #text(lang: "de", region: none, "a")
  - #text(lang: "de", region: "at", "b")
  - #text(lang: "de", region: none, "c")
- #text(lang: "de", region: none, "d")
//@@ file tests/suite/pdftags_layout.typ
--- layout-tags-placement-float pdftags ---
#set pdf(standard: "ua-1")
#set text(lang: "de")
#grid(
  columns: 2,
  [
    #set text(lang: "be")
    text
    #place(float: true, top + left)[
      `a`
    ]
  ],
  [
    text
    #place(float: true, top + left)[
      #set text(lang: "fr")
      text in grid
    ]
    text
  ],
  [
    #set text(lang: "es")
    b
  ]
)
//@@ file tests/suite/pdftags_link.typ
--- link-tags-heading-without-numbering pdftags ---
#set pdf(standard: "ua-1")
= Heading <heading>

#link(<heading>)[link to heading]

--- link-tags-heading-with-numbering pdftags ---
#set pdf(standard: "ua-1")
#set heading(numbering: "1.")
= Heading <heading>

#link(<heading>)[link to heading]

--- link-tags-non-refable-location pdftags ---
#set pdf(standard: "ua-1")
A random location <somewhere>

#link(<somewhere>)[link to somewhere]

--- link-tags-contact-prefix pdftags ---
#set pdf(standard: "ua-1")
#link("mailto:hello@typst.app")

#link("tel:123")

--- link-tags-position pdftags ---
#set pdf(standard: "ua-1")
#context link(here().position())[somewhere]

--- link-tags-link-in-artifact pdftags ---
#set pdf(standard: "ua-1")
#pdf.artifact[
  // Error: 4-42 PDF/UA-1 error: PDF artifacts may not contain links
  // Hint: 4-42 references, citations, and footnotes are also considered links in PDF
  #link("https://github.com/typst/typst")
]

--- link-tags-reference-in-artifact pdftags ---
#set pdf(standard: "ua-1")
#set heading(numbering: "1.")
= Heading <heading>
#pdf.artifact[
  // Error: 3-11 PDF/UA-1 error: PDF artifacts may not contain links
  // Hint: 3-11 references, citations, and footnotes are also considered links in PDF
  @heading
]

--- link-tags-citation-in-artifact pdftags ---
#set pdf(standard: "ua-1")
#pdf.artifact[
  // Error: 3-10 PDF/UA-1 error: PDF artifacts may not contain links
  // Hint: 3-10 references, citations, and footnotes are also considered links in PDF
  @netwok
]
#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- link-tags-with-parbreak-error pdftags ---
#set pdf(standard: "ua-1")
// Error: 7-69 PDF/UA-1 error: invalid document structure, this element's PDF tag would be split up
// Hint: 7-69 this is probably caused by paragraph grouping
// Hint: 7-69 maybe you've used a `parbreak`, `colbreak`, or `pagebreak`
Look #link("https://github.com/typst/typst")[this #parbreak() thing].

--- link-tags-with-parbreak pdftags ---
Look #link("https://github.com/typst/typst")[this #parbreak() thing].

--- link-tags-with-linebreak pdftags ---
#set pdf(standard: "ua-1")
// This forces the use of `QuadPoints`.
The following #link("https://github.com/typst/typst")[link breaks across] a line.

--- issue-7301-link-tags-empty-link-body pdftags ---
#link("asf")[#none\ #none]

--- issue-7301-link-tags-empty-link-body-linebreak pdftags ---
#link("asf", linebreak())

--- issue-7301-link-tags-empty-link-body-footnote pdftags ---
#footnote(numbering: it => "", [asdf])

--- issue-7301-link-tags-empty-link-body-mutliple pdftags ---
#link("asf")[#none\ #none] #link("asf")[#none\ #none]

--- issue-7777-missing-link-parent pdftags ---
#set text(hyphenate: true)
#block(width: 0.9cm)[#link("x")[test]ing]

--- issue-7777-missing-link-parent-repeated-hyphen pdftags ---
// In Czech and some other slavic languages hyphens are repeated at the start
// of the next line, which can cause issues with links in PDF.
#set text(lang: "cs", hyphenate: true)
#block(width: 0.9cm)[#link("x")[test-]ing]

--- link-tags-nesting-across-pars pdftags ---
This #link("https://outer.org")[is a split
#link("https://inner.org")[ multi paragraph link.

Thanks for the] multi attention.] Okay!
//@@ file tests/suite/pdftags_list.typ
--- list-tags-basic pdftags ---
#set pdf(standard: "ua-1")
- a
  - 1
- b
  - c
    - d
  - e
- f

--- list-tags-mixed-with-enum pdftags ---
#set pdf(standard: "ua-1")
- a
  + 1
- b
  + c
    - d
  + e
- f

--- list-tags-wide-with-nested-list pdftags ---
#set pdf(standard: "ua-1")
- a

  - 1

- b

  - c

    - d

  - e

- f

--- list-tags-complex-item-with-sub-list pdftags ---
#set pdf(standard: "ua-1")
- #[#quote(block: true)[hi] #footnote[1].]
  - a
  - b
- c
- d

--- list-tags-complex-item-with-nested-list pdftags ---
#set pdf(standard: "ua-1")
- #[
    #quote(block: true)[hi]
    #footnote[1].
    - a
    - b
  ]
- c
- d

--- list-tags-terms-basic pdftags ---
#set pdf(standard: "ua-1")
/ A: 1
/ B: 2
/ C: 3

--- list-tags-terms-basic-wide pdftags ---
#set pdf(standard: "ua-1")
/ A: 1

/ B: 2

/ C: 3

--- list-tags-terms-indented pdftags ---
#set pdf(standard: "ua-1")
/ A: 1
/ B: 2
  / B1: wow
  / B2: amazing

--- list-tags-terms-body-with-parbreak pdftags ---
#set pdf(standard: "ua-1")
/ A: 1 #parbreak() 232
/ B: 2

--- list-tags-terms-label-with-parbreak-error pdftags ---
#set pdf(standard: "ua-1")
// TODO: This should have the span of the term label, not the entire term item
// Error: 1-21 PDF/UA-1 error: invalid document structure, this element's PDF tag would be split up
// Hint: 1-21 this is probably caused by paragraph grouping
// Hint: 1-21 maybe you've used a `parbreak`, `colbreak`, or `pagebreak`
/ A #parbreak() A: 1
/ B: 2

--- list-tags-terms-label-with-parbreak pdftags ---
// This currently produces an empty paragraph, because terms label is moved out
// of the broken paragraph when constructing the PDF list structure. This only
// happens when tags are broken up, so it's not *that* bad.
/ A #parbreak() A: 1
/ B: 2


--- issue-7789-list-tags-breaking pdftags ---
#set list(marker: [a] + layout(layout_info => box(height: 100em)))

- A
  - A
//@@ file tests/suite/pdftags_logical-children.typ
--- logical-children-tags-place-within-artifact pdftags ---
#set pdf(standard: "ua-1")
// Error: 2:4-4:4 PDF/UA-1 error: PDF artifacts may not contain links
// Hint: 2:4-4:4 references, citations, and footnotes are also considered links in PDF
#pdf.artifact[
  #link("tel:123")[
    #place(float: true, top + left, rect(fill: red))
  ]
]

--- logical-children-tags-link-within-place-within-artifact pdftags ---
#set pdf(standard: "ua-1")
// Error: 3:6-5:6 PDF/UA-1 error: PDF artifacts may not contain links
// Hint: 3:6-5:6 references, citations, and footnotes are also considered links in PDF
#pdf.artifact[
  #place(float: true, top + left)[
    #link("tel:123")[
      #rect(fill: red)
    ]
  ]
]

--- logical-children-tags-footnote-in-tiling pdftags ---
#set pdf(standard: "ua-1")
// Error: PDF/UA-1 error: PDF artifacts may not contain links
// Hint: a link was used within a tiling
// Hint: references, citations, and footnotes are also considered links in PDF
#rect(width: 90pt, height: 90pt, fill: tiling(size: (30pt, 30pt))[
  #footnote[hi]
])

--- logical-children-tags-place-in-tiling pdftags empty ---
#set pdf(standard: "ua-1")
#rect(width: 90pt, height: 90pt, fill: tiling(size: (30pt, 30pt))[
  #place(float: true, top + right)[hi]
])

--- logical-children-tags-decorations-in-broken-grid-cell pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 50pt)
#grid(
  columns: 2,
  underline[
    #lorem(10)
  ],
  overline[
    #lorem(10)
  ],
)

--- logical-children-tags-hide-around-footnote pdftags ---
#set pdf(standard: "ua-1")
#hide[
  Some text #footnote[explanation].
]

Some other text.

--- logical-children-tags-hide-around-place pdftags ---
#set pdf(standard: "ua-1")
#hide[
  Some text #place(float: true, bottom + right)[explanation].
]

Some other text.


--- logical-children-tags-underline-around-footnote pdftags ---
#set pdf(standard: "ua-1")
#underline[
  Some text #footnote[explanation].
]

Some other text.

--- logical-children-tags-underline-around-place paged pdftags ---
#set pdf(standard: "ua-1")
#underline[
  Some text #place(float: true, bottom + right)[explanation].
]

Some other text.
//@@ file tests/suite/pdftags_query.typ
--- query-tags-duplicate-heading pdftags ---
#set pdf(standard: "ua-1")
// This will display the heading with the same location a second time
#context query(heading).join()
= Hi

--- query-tags-duplicate-labelled-element pdftags ---
#set pdf(standard: "ua-1")
#figure(alt: "Text saying: hello there")[
  hello there
] <figure>

#context query(<figure>).at(0)

--- query-tags-ambiguous-parent-place-error pdftags ---
#set pdf(standard: "ua-1")
// Error: 2-43 PDF/UA-1 error: ambiguous logical parent
// Hint: 2-43 please report this as a bug
#place(float: true, top + left)[something] <placed>

#context query(<placed>).join()

--- query-tags-ambiguous-parent-place pdftags ---
#place(float: true, top + left)[something] <placed>

#context query(<placed>).join()

--- query-tags-ambiguous-parent-footnote-error pdftags ---
#set pdf(standard: "ua-1")
// Error: 2-21 PDF/UA-1 error: ambiguous logical parent
// Hint: 2-21 please report this as a bug
#footnote[something] <note>

#context query(<note>).join()

--- query-tags-ambiguous-parent-footnote pdftags ---
#footnote[something] <note>

#context query(<note>).join()
//@@ file tests/suite/pdftags_raw.typ
--- raw-tags-basic-block pdftags ---
#set pdf(standard: "ua-1")
```rs
const PDF_STANDARD: &str = "PDF/UA-1";

fn main() {
    println!("hello {PDF_STANDARD}");
}
```

--- raw-tags-basic-inline pdftags ---
#set pdf(standard: "ua-1")
Some `inline raw` text and the rust ```rs fn``` keyword.
//@@ file tests/suite/pdftags_table.typ
--- table-tags-empty pdftags ---
#set pdf(standard: "ua-1")
#table()

--- table-tags-basic pdftags ---
#set pdf(standard: "ua-1")
#table(
  columns: 3,
  table.header([H1], [H2], [H3]),
  [a1], [a2], [a3],
  [b1], [b2], [b3],
)

--- table-tags-column-and-row-header pdftags ---
#set pdf(standard: "ua-1")
#table(
  columns: 3,
  table.header([H1], [H2], [H3]),
  pdf.header-cell(scope: "row")[10:00], [a2], [a3],
  pdf.header-cell(scope: "row")[12:30], [b2], [b3],
)

--- table-tags-missing-cells pdftags ---
#set pdf(standard: "ua-1")
#table(
  columns: 3,
  table.header(level: 1, [H1], [H1], [H1]),
  table.header(level: 2, [H2], [H2], [H2]),

  // the middle cell is missing
  table.cell(x: 0)[],
  table.cell(x: 2)[],

  // the last cell is missing, its type should be inferred from the row
  table.header(level: 2, [H2], [H2]),

  // last cell is missing
  [], [],

  table.footer(
    table.cell(x: 1)[F],
    table.cell(x: 2)[F],
  ),
)

--- table-tags-explicit-lines pdftags ---
#set pdf(standard: "ua-1")
#table(
  columns: 2,
  [a], table.vline(stroke: green), [b],
  table.hline(stroke: red),
  [c], [d],
  table.hline(stroke: blue),
)

--- table-tags-unset-bottom-line pdftags ---
#set pdf(standard: "ua-1")
#table(
  columns: 2,
  [a], [b],
  [c], [d],
  table.hline(stroke: none),
)

--- table-tags-different-default-border pdftags ---
#set pdf(standard: "ua-1")
#table(
  columns: 2,
  stroke: red + 2pt,
  table.hline(stroke: black),
  [a], [b],
  [c], [d],
  [e], [f],
  table.hline(stroke: black),
)

--- table-tags-show-rule-error pdftags ---
#set pdf(standard: "ua-1")
#set table(columns: (10pt, auto))
// Error: 2-30 PDF/UA-1 error: invalid table (Table) structure
// Hint: 2-30 table (Table) may not contain raw text (Code)
// Hint: 2-30 this is probably caused by a show rule
#show table: it => it.columns
#table[A][B][C][D]

--- table-tags-show-rule pdftags ---
#set table(columns: (10pt, auto))
#show table: it => it.columns
#table[A][B][C][D]

--- table-tags-rowspan-split-1 pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 6em)
#table(
  rows: (4em, auto, 4em),
  columns: 2,
  table.cell(rowspan: 3, [a\ ] * 4),
  [b], [c], [d],
)

--- table-tags-rowspan-split-2 pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 6em)
#table(
  rows: (4em, auto, 4em),
  columns: 3,
  [a1], table.cell(rowspan: 3, [b\ ] * 5), [c1],
  [a2], [c2],
  [a3], [c3],
)

--- table-tags-citation-in-repeated-header pdftags ---
#set pdf(standard: "ua-1")
#set page(height: 60pt)
#table(
  // Error: 16-23 PDF/UA-1 error: PDF artifacts may not contain links
  // Hint: 16-23 references, citations, and footnotes are also considered links in PDF
  table.header[@netwok],
  [A],
  [A],
)

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- table-tags-private-table-summary eval ---
#set pdf(standard: "ua-1")
#table(
  // Error: 3-18 unexpected argument: summary
  summary: "nope",
  [A],
)

--- table-tags-private-cell-kind eval ---
#set pdf(standard: "ua-1")
// Error: 13-25 unexpected argument: kind
#table.cell(kind: "nope")[A]

--- table-tags-unstable-functions pdftags ---
#set pdf(standard: "ua-1")
#pdf.table-summary(
  summary: "The table summary",
  table(
    columns: 2,
    // Exclude the top-left cell from being a header cell.
    table.header(pdf.data-cell[], [Column header]),
    pdf.header-cell(scope: "row")[Row header], [thing]
  )
)
//@@ file tests/suite/pdftags_text.typ
--- issue-7763-text-tags-span-order pdftags ---
#set pdf(standard: "ua-1")
#stack[A text *outside* of _a paragraph_]
//@@ file tests/suite/playground.typ
--- playground eval ---
//@@ file tests/suite/scripting_blocks.typ
// Test code blocks.

--- code-block-basic-syntax paged ---

// Evaluates to join of none, [My ] and the two loop bodies.
#{
  let parts = ("my fri", "end.")
  [Hello, ]
  for s in parts [#s]
}

// Evaluates to join of the content and strings.
#{
  [How]
  if true {
    " are"
  }
  [ ]
  if false [Nope]
  [you] + "?"
}

--- code-block-empty eval ---
// Nothing evaluates to none.
#test({}, none)

--- code-block-let eval ---
// Let evaluates to none.
#test({ let v = 0 }, none)

--- code-block-single-expression eval ---
// Evaluates to single expression.
#test({ "hello" }, "hello")

--- code-block-multiple-expressions-single-line eval ---
// Evaluates to string.
#test({ let x = "m"; x + "y" }, "my")

--- code-block-join-let-with-expression eval ---
// Evaluated to int.
#test({
  let x = 1
  let y = 2
  x + y
}, 3)

--- code-block-join-expression-with-none eval ---
// String is joined with trailing none, evaluates to string.
#test({
  type("")
  none
}, str)

--- code-block-join-int-with-content eval ---
// Some things can't be joined.
#{
  [A]
  // Error: 3-4 cannot join content with integer
  1
  [B]
}

--- code-block-scope-in-markup eval ---
// Block directly in markup also creates a scope.
#{ let x = 1 }

// Error: 7-8 unknown variable `x`
#test(x, 1)

--- code-block-scope-in-let eval ---
// Block in expression does create a scope.
#let a = {
  let b = 1
  b
}

#test(a, 1)

// Error: 3-4 unknown variable `b`
#{b}

--- code-block-double-scope eval ---
// Double block creates a scope.
#{{
  import "module.typ": b
  test(b, 1)
}}

// Error: 2-3 unknown variable `b`
#b

--- code-block-nested-scopes eval ---
// Multiple nested scopes.
#{
  let a = "a1"
  {
    let a = "a2"
    {
      test(a, "a2")
      let a = "a3"
      test(a, "a3")
    }
    test(a, "a2")
  }
  test(a, "a1")
}

--- code-block-multiple-literals-without-semicolon eval ---
// Multiple unseparated expressions in one line.
// Error: 4 expected semicolon or line break
#{1 2}

--- code-block-multiple-expressions-without-semicolon eval ---
// Error: 13 expected semicolon or line break
// Error: 23 expected semicolon or line break
#{let x = -1 let y = 3 x + y}

--- code-block-incomplete-expressions eval ---
#{
  // Error: 7-10 expected pattern, found string
  for "v"

  // Error: 8 expected keyword `in`
  // Error: 22 expected block
  for v let z = 1 + 2

  z
}

--- code-block-unclosed eval ---
// Error: 2-3 unclosed delimiter
#{

--- code-block-unopened eval ---
// Error: 2-3 unexpected closing brace
#}

--- single-right-bracket eval ---
// Error: 1-2 unexpected closing bracket
// Hint: 1-2 try using a backslash escape: \]
]

--- right-bracket-nesting eval ---
[
= [ Hi ]]
- how [
  - are ]
// Error: 10-11 unexpected closing bracket
// Hint: 10-11 try using a backslash escape: \]
  - error][]
[[]]

--- right-bracket-hash eval ---
// Error: 2-3 unexpected closing bracket
#]

--- right-bracket-in-blocks eval ---
// Error: 3-4 unclosed delimiter
// Error: 6-7 unexpected closing bracket
// Hint: 6-7 try using a backslash escape: \]
[#{]}]

// Error: 4-5 unexpected closing bracket
// Hint: 4-5 try using a backslash escape: \]
#[]]

// Error: 4-5 unclosed delimiter
// Error: 7-8 unexpected closing bracket
// Hint: 7-8 try using a backslash escape: \]
#[#{]}]

// Error: 2-3 unclosed delimiter
// Error: 3-4 unclosed delimiter
// Error: 4-5 unexpected closing bracket
// Hint: 4-5 try using a backslash escape: \]
#{{]}}

--- content-block-in-markup-scope eval ---
// Content blocks also create a scope.
#[#let x = 1]

// Error: 2-3 unknown variable `x`
#x
//@@ file tests/suite/scripting_call.typ
// Test function calls.

--- call-basic paged ---

// Omitted space.
#let f() = {}
#[#f()*Bold*]

// Call return value of function with body.
#let f(x, body) = (y) => [#x] + body + [#y]
#f(1)[2](3)

// Don't parse this as a function.
#test (it)

#let f(body) = body
#f[A]
#f()[A]
#f([A])

#let g(a, b) = a + b
#g[A][B]
#g([A], [B])
#g()[A][B]

--- call-aliased-function eval ---
// Call function assigned to variable.
#let alias = type
#test(alias(alias), type)

--- call-complex-callee-expression eval ---
// Callee expressions.
#{
  // Wrapped in parens.
  test((type)("hi"), str)

  // Call the return value of a function.
  let adder(dx) = x => x + dx
  test(adder(2)(5), 7)
}

--- call-bad-type-bool-literal eval ---
// Error: 2-6 expected function, found boolean
#true()

--- call-bad-type-string-var eval ---
#let x = "x"

// Error: 2-3 expected function, found string
#x()

--- call-shadowed-builtin-function eval ---
#let image = "image"

// Error: 2-7 expected function, found string
// Hint: 2-7 use `std.image` to access the shadowed standard library function
#image("image")

--- call-bad-type-int-expr eval ---
#let f(x) = x

// Error: 2-6 expected function, found integer
#f(1)(2)

--- call-bad-type-content-expr eval ---
#let f(x) = x

// Error: 2-6 expected function, found content
#f[1](2)

--- call-args-trailing-comma eval ---
// Trailing comma.
#test(1 + 1, 2,)

--- call-args-duplicate eval ---
// Error: 26-30 duplicate argument: font
#set text(font: "Arial", font: "Helvetica")

--- call-args-bad-positional-as-named eval ---
// Error: 4-15 the argument `amount` is positional
// Hint: 4-15 try removing `amount:`
#h(amount: 0.5)

--- call-args-bad-colon eval ---
// Error: 7-8 unexpected colon
#func(:)

--- call-args-bad-token eval ---
// Error: 10-12 unexpected end of block comment
// Hint: 10-12 consider escaping the `*` with a backslash or opening the block comment with `/*`
#func(a:1*/)

--- call-args-missing-comma eval ---
// Error: 8 expected comma
#func(1 2)

--- call-args-bad-name-and-incomplete-pair eval ---
// Error: 7-8 expected identifier, found integer
// Error: 9 expected expression
#func(1:)

--- call-args-bad-name-int eval ---
// Error: 7-8 expected identifier, found integer
#func(1:2)

--- call-args-bad-name-string eval ---
// Error: 7-12 expected identifier, found string
#func("abc": 2)

--- call-args-bad-name-group eval ---
// Error: 7-10 expected identifier, found group
#func((x):1)

--- call-args-lone-underscore eval ---
// Test that lone underscore works.
#test((1, 2, 3).map(_ => {}).len(), 3)

--- call-args-spread-override eval ---
// Test standard argument overriding.
#{
  let f(style: "normal", weight: "regular") = {
    "(style: " + style + ", weight: " + weight + ")"
  }

  let myf(..args) = f(weight: "bold", ..args)
  test(myf(), "(style: normal, weight: bold)")
  test(myf(weight: "black"), "(style: normal, weight: black)")
  test(myf(style: "italic"), "(style: italic, weight: bold)")
}

--- call-args-spread-forward eval ---
// Test multiple calls.
#{
  let f(b, c: "!") = b + c
  let g(a, ..sink) = a + f(..sink)
  test(g("a", "b", c: "c"), "abc")
}

--- call-args-spread-type-repr eval ---
// Test doing things with arguments.
#{
  let save(..args) = {
    test(type(args), arguments)
    test(repr(args), "arguments(three: true, 1, 2)")
  }

  save(1, 2, three: true)
}

--- call-args-spread-array-and-dict eval ---
// Test spreading array and dictionary.
#{
  let more = (3, -3, 6, 10)
  test(calc.min(1, 2, ..more), -3)
  test(calc.max(..more, 9), 10)
  test(calc.max(..more, 11), 11)
}

#{
  let more = (c: 3, d: 4)
  let tostr(..args) = repr(args)
  test(tostr(a: 1, ..more, b: 2), "arguments(a: 1, c: 3, d: 4, b: 2)")
}

--- call-args-spread-none eval ---
// None is spreadable.
#let f() = none
#f(..none)
#f(..if false {})
#f(..for x in () [])

--- call-args-spread-string-invalid eval ---
// Error: 11-19 cannot spread string
#calc.min(.."nope")

--- call-args-content-block-unclosed eval ---
// Error: 6-7 unclosed delimiter
#func[`a]`

--- issue-886-args-sink paged ---
// Test bugs with argument sinks.
#let foo(..body) = repr(body.pos())
#foo(a: "1", b: "2", 1, 2, 3, 4, 5, 6)

--- issue-3144-unexpected-arrow eval ---
#let f(a: 10) = a(1) + 1
#test(f(a: _ => 5), 6)

--- issue-3502-space-and-comments-around-destructuring-colon eval ---
#let ( key :  /* hi */ binding ) = ( key: "ok" )
#test(binding, "ok")

--- issue-3502-space-around-dict-colon eval ---
#test(( key : "value" ).key, "value")

--- issue-3502-space-around-param-colon eval ---
// Test that a space after a named parameter is permissible.
#let f( param : v ) = param
#test(f( param /* ok */ : 2 ), 2)

--- call-feature-gated eval features() ---
// Error: 2-6 cannot access variable `html` because the `html` feature is not enabled
// Hint: 2-6 try enabling the `html` feature
// Hint: 2-6 see https://typst.app/help/compiler-features for more details
#html()

--- call-args-unclosed eval ---
// Error: 7-8 unclosed delimiter
#{func(}

--- call-args-unclosed-string eval ---
// Error: 6-7 unclosed delimiter
// Error: 1:7-2:1 unclosed string
#func("]
//@@ file tests/suite/scripting_closure.typ
// Test closures.

--- closure-without-params-non-atomic paged ---
// Don't parse closure directly in content.

#let x = "x"

// Should output `x => y`.
#x => y

--- closure-without-captures eval ---
// Basic closure without captures.
#{
  let adder = (x, y) => x + y
  test(adder(2, 3), 5)
}

--- closure-as-arg eval ---
// Pass closure as argument and return closure.
// Also uses shorthand syntax for a single argument.
#{
  let chain = (f, g) => (x) => f(g(x))
  let f = x => x + 1
  let g = x => 2 * x
  let h = chain(f, g)
  test(h(2), 5)
}

--- closure-capture-from-popped-stack-frame eval ---
// Capture environment.
#{
  let mark = "!"
  let greet = {
    let hi = "Hi"
    name => {
        hi + ", " + name + mark
    }
  }

  test(greet("Typst"), "Hi, Typst!")

  // Changing the captured variable after the closure definition has no effect.
  mark = "?"
  test(greet("Typst"), "Hi, Typst!")
}

--- closure-shadows-outer-var eval ---
// Redefined variable.
#{
  let x = 1
  let f() = {
    let x = x + 2
    x
  }
  test(f(), 3)
}

--- closure-shadows-outer-var-import eval ---
// Import bindings.
#{
  let b = "module.typ"
  let f() = {
    import b: b
    b
  }
  test(f(), 1)
}

--- closure-shadows-outer-var-for-loop eval ---
// For loop bindings.
#{
  let v = (1, 2, 3)
  let f() = {
    let s = 0
    for v in v { s += v }
    s
  }
  test(f(), 6)
}

--- closure-let-basic eval ---
// Let + closure bindings.
#{
  let g = "hi"
  let f() = {
    let g() = "bye"
    g()
  }
  test(f(), "bye")
}

--- closure-let-args eval ---
// Parameter bindings.
#{
  let x = 5
  let g() = {
    let f(x, y: x) = x + y
    f
  }

  test(g()(8), 13)
}

--- closure-bad-capture eval ---
// Don't leak environment.
#{
  // Error: 16-17 unknown variable `x`
  let func() = x
  let x = "hi"
  func()
}

--- closure-missing-arg-positional eval ---
// Too few arguments.
#{
  let types(x, y) = "[" + str(type(x)) + ", " + str(type(y)) + "]"
  test(types(14%, 12pt), "[ratio, length]")

  // Error: 8-21 missing argument: y
  test(types("nope"), "[string, none]")
}

--- closure-too-many-args-positional eval ---
// Too many arguments.
#{
  let f(x) = x + 1

  // Error: 8-13 unexpected argument
  f(1, "two", () => x)
}

--- closure-capture-in-lvalue paged ---
// Mutable method with capture in argument.
#let x = "b"
#let f() = {
  let a = (b: 5)
  a.at(x) = 10
  a
}
#f()

--- closure-capture-mut-error eval ---
#let x = ()
#let f() = {
  // Error: 3-4 variables from outside the function are read-only and cannot be modified
  x.at(1) = 2
}
#f()

--- closure-capture-mut-false-positive eval ---
// Test the closure capture error on a non-mutating field call.
#let sx = symbol("p", ("push", sym.tilde))
#let f() = {
  // Error: 3-5 variables from outside the function are read-only and cannot be modified
  sx.push(none)
}
#test(f(), std.sym.tilde(none))

--- closure-named-args-basic eval ---
// Named arguments.
#{
  let greet(name, birthday: false) = {
    if birthday { "Happy Birthday, " } else { "Hey, " } + name + "!"
  }

  test(greet("Typst"), "Hey, Typst!")
  test(greet("Typst", birthday: true), "Happy Birthday, Typst!")

  // Error: 23-35 unexpected argument: whatever
  test(greet("Typst", whatever: 10))
}

--- closure-args-sink eval ---
// Parameter unpacking.
#let f((a, b), ..c) = (a, b, c)
#test(f((1, 2), 3, 4), (1, 2, (3, 4)))

#let f((k: a, b), c: 3, (d,)) = (a, b, c, d)
#test(f((k: 1, b: 2), (4,)), (1, 2, 3, 4))

// Error: 8-14 expected identifier, found destructuring pattern
#let f((a, b): 0) = none

// Error: 10-19 expected pattern, found array
#let f(..(a, b: c)) = none

// Error: 10-16 expected pattern, found array
#let f(..(a, b)) = none

--- closure-param-duplicate-positional eval ---
// Error: 11-12 duplicate parameter: x
#let f(x, x) = none

--- closure-body-multiple-expressions eval ---
// Error: 21 expected comma
// Error: 22-23 expected pattern, found integer
// Error: 24-25 unexpected plus
// Error: 26-27 expected pattern, found integer
#let f = (x: () => 1 2 + 3) => 4

--- closure-param-duplicate-mixed eval ---
// Error: 14-15 duplicate parameter: a
// Error: 23-24 duplicate parameter: b
// Error: 35-36 duplicate parameter: b
#let f(a, b, a: none, b: none, c, b) = none

--- closure-param-duplicate-spread eval ---
// Error: 13-14 duplicate parameter: a
#let f(a, ..a) = none

--- closure-pattern-bad-string eval ---
// Error: 7-14 expected pattern, found string
#((a, "named": b) => none)

--- closure-let-pattern-bad-string eval ---
// Error: 10-15 expected pattern, found string
#let foo("key": b) = key

--- closure-param-keyword eval ---
// Error: 10-14 expected pattern, found `none`
// Hint: 10-14 keyword `none` is not allowed as an identifier; try `none_` instead
#let foo(none: b) = key

--- closure-param-named-underscore eval ---
// Error: 10-11 expected identifier, found underscore
#let foo(_: 3) = none

--- issue-non-atomic-closure paged ---
// Ensure that we can't have non-atomic closures.
#let x = 1
#let c = [#(x) => (1, 2)]
#test(c.children.last(), [(1, 2)]))
//@@ file tests/suite/scripting_destructuring.typ
--- destructuring-group-1 eval ---
// This wasn't allowed.
#let ((x)) = 1
#test(x, 1)

--- destructuring-group-2 eval ---
// This also wasn't allowed.
#let ((a, b)) = (1, 2)
#test(a, 1)
#test(b, 2)

--- destructuring-dict-underscore eval ---
// Here, `best` was accessed as a variable, where it shouldn't have.
#{
  (best: _) = (best: "brr")
}

--- destructuring-dict-array-at eval ---
// Same here.
#{
  let array = (1, 2, 3, 4)
  (test: array.at(1), best: _) = (test: "baz", best: "brr")
  test(array, (1, "baz", 3, 4))
}

--- destructuring-dict-bad eval ---
// Error: 7-10 expected identifier, found group
// Error: 12-14 expected pattern, found integer
#let ((a): 10) = "world"

--- destructuring-bad-duplicate eval ---
// Here, `a` is not duplicate, where it was previously identified as one.
#let f((a: b), (c,), a) = (a, b, c)
#test(f((a: 1), (2,), 3), (3, 1, 2))

--- destructuring-non-atomic eval ---
// Ensure that we can't have non-atomic destructuring.
#let x = 1
#let c = [#() = ()]
#test(c.children.last(), [()])

--- destructuring-let-array eval ---
// Simple destructuring.
#let (a, b) = (1, 2)
#test(a, 1)
#test(b, 2)

--- destructuring-let-array-single-item eval ---
#let (a,) = (1,)
#test(a, 1)

--- destructuring-let-array-placeholders eval ---
// Destructuring with multiple placeholders.
#let (a, _, c, _) = (1, 2, 3, 4)
#test(a, 1)
#test(c, 3)

--- destructuring-let-array-with-sink-at-end eval ---
// Destructuring with a sink.
#let (a, b, ..c) = (1, 2, 3, 4, 5, 6)
#test(a, 1)
#test(b, 2)
#test(c, (3, 4, 5, 6))

--- destructuring-let-array-with-sink-in-middle eval ---
// Destructuring with a sink in the middle.
#let (a, ..b, c) = (1, 2, 3, 4, 5, 6)
#test(a, 1)
#test(b, (2, 3, 4, 5))
#test(c, 6)

--- destructuring-let-array-with-sink-at-start-empty eval ---
// Destructuring with an empty sink.
#let (..a, b, c) = (1, 2)
#test(a, ())
#test(b, 1)
#test(c, 2)

--- destructuring-let-array-with-sink-in-middle-empty eval ---
// Destructuring with an empty sink.
#let (a, ..b, c) = (1, 2)
#test(a, 1)
#test(b, ())
#test(c, 2)

--- destructuring-let-array-with-sink-at-end-empty eval ---
// Destructuring with an empty sink.
#let (a, b, ..c) = (1, 2)
#test(a, 1)
#test(b, 2)
#test(c, ())

--- destructuring-let-array-with-sink-empty eval ---
// Destructuring with an empty sink and empty array.
#let (..a) = ()
#test(a, ())

--- destructuring-let-array-with-unnamed-sink eval ---
// Destructuring with unnamed sink.
#let (a, .., b) = (1, 2, 3, 4)
#test(a, 1)
#test(b, 4)

// Error: 10-11 duplicate binding: a
#let (a, a) = (1, 2)

// Error: 12-15 only one destructuring sink is allowed
#let (..a, ..a) = (1, 2)

// Error: 12-13 duplicate binding: a
#let (a, ..a) = (1, 2)

// Error: 13-14 duplicate binding: a
#let (a: a, a) = (a: 1, b: 2)

// Error: 13-20 expected pattern, found function call
#let (a, b: b.at(0)) = (a: 1, b: 2)

// Error: 7-14 expected pattern, found function call
#let (a.at(0),) = (1,)

--- destructuring-let-empty-array eval ---
#let () = ()

--- destructuring-let-empty-array-too-many-elements eval ---
// Error: 6-8 too many elements to destructure
// Hint: 6-8 the provided array has a length of 2, but the pattern expects an empty array
#let () = (1, 2)

--- destructuring-let-array-too-few-elements eval ---
// Error: 6-15 not enough elements to destructure
// Hint: 6-15 the provided array has a length of 2, but the pattern expects 3 elements
#let (a, b, c) = (1, 2)

--- destructuring-let-array-too-few-elements-with-sink eval ---
// Error: 6-20 not enough elements to destructure
// Hint: 6-20 the provided array has a length of 2, but the pattern expects at least 3 elements
#let (..a, b, c, d) = (1, 2)

--- destructuring-let-array-too-few-elements-with-sink-1-element eval ---
// Error: 6-14 not enough elements to destructure
// Hint: 6-14 the provided array has a length of 0, but the pattern expects at least 1 element
#let (..a, b) = ()

--- destructuring-let-array-bool-invalid eval ---
// Error: 6-12 cannot destructure boolean
#let (a, b) = true

--- destructuring-let-dict eval ---
// Simple destructuring.
#let (a: a, b, x: c) = (a: 1, b: 2, x: 3)
#test(a, 1)
#test(b, 2)
#test(c, 3)

--- destructuring-let-dict-with-sink-at-end eval ---
// Destructuring with a sink.
#let (a: _, ..b) = (a: 1, b: 2, c: 3)
#test(b, (b: 2, c: 3))

--- destructuring-let-dict-with-sink-in-middle eval ---
// Destructuring with a sink in the middle.
#let (a: _, ..b, c: _) = (a: 1, b: 2, c: 3)
#test(b, (b: 2))

--- destructuring-let-dict-with-sink-at-end-empty eval ---
// Destructuring with an empty sink.
#let (a: _, ..b) = (a: 1)
#test(b, (:))

--- destructuring-let-dict-with-sink-empty eval ---
// Destructuring with an empty sink and empty dict.
#let (..a) = (:)
#test(a, (:))

--- destructuring-let-dict-with-unnamed-sink eval ---
// Destructuring with unnamed sink.
#let (a, ..) = (a: 1, b: 2)
#test(a, 1)

--- destructuring-let-nested eval ---
// Nested destructuring.
#let ((a, b), (key: c)) = ((1, 2), (key: 3))
#test((a, b, c), (1, 2, 3))

--- destructuring-let-dict-key-string-invalid eval ---
// Keyed destructuring is not currently supported.
// Error: 7-18 expected pattern, found string
#let ("spacy key": val) = ("spacy key": 123)
#val

--- destructuring-let-dict-key-expr-invalid eval ---
// Keyed destructuring is not currently supported.
#let x = "spacy key"
// Error: 7-10 expected identifier, found group
#let ((x): v) = ("spacy key": 123)

--- destructuring-let-array-trailing-placeholders eval ---
// Trailing placeholders.
// Error: 6-21 not enough elements to destructure
// Hint: 6-21 the provided array has a length of 1, but the pattern expects 5 elements
#let (a, _, _, _, _) = (1,)
#test(a, 1)

--- destructuring-let-dict-patterns-invalid eval ---
// Error: 10-13 expected pattern, found string
// Error: 18-19 expected pattern, found integer
#let (a: "a", b: 2) = (a: 1, b: 2)

--- destructuring-let-dict-shorthand-missing-key eval ---
// Error: 10-11 dictionary does not contain key "b"
#let (a, b) = (a: 1)

--- destructuring-let-dict-missing-key eval ---
// Error: 10-11 dictionary does not contain key "b"
#let (a, b: b) = (a: 1)

--- destructuring-let-dict-from-array eval ---
// Error: 7-11 cannot destructure named pattern from an array
#let (a: a, b) = (1, 2, 3)

--- destructuring-during-loop-continue paged ---
// Test continue while destructuring.
// Should output "one = I \ two = II \ one = I".
#for num in (1, 2, 3, 1) {
  let (word, roman) = if num == 1 {
    ("one", "I")
  } else if num == 2 {
    ("two", "II")
  } else {
    continue
  }
  [#word = #roman \ ]
}

--- destructuring-assign eval ---
// Test destructuring assignments.

#let a = none
#let b = none
#let c = none
#((a,) = (1,))
#test(a, 1)

#((_, a, b, _) = (1, 2, 3, 4))
#test(a, 2)
#test(b, 3)

#((a, b, ..c) = (1, 2, 3, 4, 5, 6))
#test(a, 1)
#test(b, 2)
#test(c, (3, 4, 5, 6))

#((a: a, b, x: c) = (a: 1, b: 2, x: 3))
#test(a, 1)
#test(b, 2)
#test(c, 3)

#let a = (1, 2)
#((a: a.at(0), b) = (a: 3, b: 4))
#test(a, (3, 2))
#test(b, 4)

#let a = (1, 2)
#((a.at(0), b) = (3, 4))
#test(a, (3, 2))
#test(b, 4)

#((a, ..b) = (1, 2, 3, 4))
#test(a, 1)
#test(b, (2, 3, 4))

#let a = (1, 2)
#((b, ..a.at(0)) = (1, 2, 3, 4))
#test(a, ((2, 3, 4), 2))
#test(b, 1)

--- destructuring-assign-commas eval ---
// Test comma placement in destructuring assignment.
#let array = (1, 2, 3)
#((key: array.at(1)) = (key: "hi"))
#test(array, (1, "hi", 3))

#let array = (1, 2, 3)
#((array.at(1)) = ("hi"))
#test(array, (1, "hi", 3))

#let array = (1, 2, 3)
#((array.at(1),) = ("hi",))
#test(array, (1, "hi", 3))

#let array = (1, 2, 3)
#((array.at(1)) = ("hi",))
#test(array, (1, ("hi",), 3))

--- destructuring-assign-nested eval ---
// Test nested destructuring assignment.
#let a
#let b
#let c
#(((a, b), (key: c)) = ((1, 2), (key: 3)))
#test((a, b, c), (1, 2, 3))

--- destructuring-assign-nested-invalid eval ---
#let array = (1, 2, 3)
// Error: 3-17 cannot destructure string
#((array.at(1),) = ("hi"))
#test(array, (1, ("hi",), 3))

--- issue-3275-normal-variable eval ---
// Normal variable.
#for x in (1, 2) {}
#for x in (a: 1, b: 2) {}
#for x in "foo" {}
#for x in bytes("😊") {}

--- issue-3275-placeholder eval ---
// Placeholder.
#for _ in (1, 2) {}
#for _ in (a: 1, b: 2) {}
#for _ in "foo" {}
#for _ in bytes("😊") {}

--- issue-3275-destructuring eval ---
// Destructuring.
#for (a,b,c) in (("a", 1, bytes(())), ("b", 2, bytes(""))) {}
#for (a, ..) in (("a", 1, bytes(())), ("b", 2, bytes(""))) {}
#for (k, v)  in (a: 1, b: 2, c: 3) {}
#for (.., v) in (a: 1, b: 2, c: 3) {}

--- issue-3275-loop-over-content eval ---
// Error: 11-17 cannot loop over content
#for x in [1, 2] {}

--- issue-3275-loop-over-arguments eval ---
// Error: 11-25 cannot loop over arguments
#for _ in arguments("a") {}

--- issue-3275-loop-over-integer eval ---
// Error: 16-21 cannot loop over integer
#for (x, y) in 12306 {}

--- issue-3275-destructuring-loop-over-content eval ---
// Error: 16-22 cannot loop over content
#for (x, y) in [1, 2] {}

--- issue-3275-destructuring-loop-over-string eval ---
// Error: 6-12 cannot destructure values of string
#for (x, y) in "foo" {}

--- issue-3275-destructuring-loop-over-string-array eval ---
// Error: 6-12 cannot destructure string
#for (x, y) in ("foo", "bar") {}

--- issue-3275-destructuring-loop-over-bytes eval ---
// Error: 6-12 cannot destructure values of bytes
#for (x, y) in bytes("😊") {}

--- issue-3275-destructuring-loop-over-bytes-array eval ---
// Error: 6-12 cannot destructure bytes
#for (x, y) in (bytes((1,2)), bytes((1,2))) {}

--- issue-3275-destructuring-loop-over-int-array eval ---
// Error: 6-12 cannot destructure integer
#for (x, y) in (1, 2) {}

--- issue-3275-destructuring-loop-over-2d-array-1 eval ---
// Error: 6-12 not enough elements to destructure
// Hint: 6-12 the provided array has a length of 1, but the pattern expects 2 elements
#for (x, y) in ((1,), (2,)) {}

--- issue-3275-destructuring-loop-over-2d-array-2 eval ---
// Error: 6-12 too many elements to destructure
// Hint: 6-12 the provided array has a length of 3, but the pattern expects 2 elements
#for (x, y) in ((1,2,3), (4,5,6)) {}

--- issue-4573-destructuring-unclosed-delimiter eval ---
// Tests a case where parsing within an incorrectly predicted paren expression
// (the "outer" assignment) would put the parser in an invalid state when
// reloading a stored prediction (the "inner" assignment) and cause a panic when
// generating the unclosed delimiter error. See the comment in the issue for
// more details.
#{
  (
    // Error: 5-7 expected pattern, found keyword `if`
    // Hint: 5-7 keyword `if` is not allowed as an identifier; try `if_` instead
    // Error: 9 expected comma
    // Error: 13-17 unexpected keyword `else`
    // Error: 20 expected comma
    if x {} else {}
    // Error: 5-6 unclosed delimiter
    { () = "inner"
  ) = "outer"
}

//@@ file tests/suite/scripting_field.typ
// Test field access.

--- field-function paged ---
// Test fields on function scopes.
#enum.item
#assert.eq
#assert.ne

--- field-normal-function-invalid eval ---
// Error: 9-16 function `assert` does not contain field `invalid`
#assert.invalid

--- field-elem-function-invalid eval ---
// Error: 7-14 function `enum` does not contain field `invalid`
#enum.invalid

--- field-elem-function-invalid-call eval ---
// Error: 7-14 function `enum` does not contain field `invalid`
#enum.invalid()

--- field-closure-invalid eval ---
// Closures cannot have fields.
#let f(x) = x
// Error: 4-11 cannot access fields on user-defined functions
#f.invalid

--- field-bool-invalid eval ---
// Error: 8-10 cannot access fields on type boolean
#false.ok

--- field-bool-keyword-invalid eval ---
// Error: 9-13 cannot access fields on type boolean
#{false.true}

--- field-invalid-none eval ---
#{
  let object = none
  // Error: 3-9 none does not have accessible fields
  object.property = "value"
}

--- field-invalid-int eval ---
#{
  let object = 10
  // Error: 3-9 integer does not have accessible fields
  object.property = "value"
}

--- field-mutable-invalid-symbol eval ---
#{
  let object = sym.eq.not
  // Error: 3-9 cannot mutate fields on symbol
  object.property = "value"
}

--- field-mutable-invalid-module eval ---
#{
  let object = calc
  // Error: 3-9 cannot mutate fields on module
  object.property = "value"
}

--- field-mutable-invalid-function eval ---
#{
  let object = calc.sin
  // Error: 3-9 cannot mutate fields on function
  object.property = "value"
}

--- field-mutable-invalid-stroke eval ---
#{
  let s = 1pt + red
  // Error: 3-4 fields on stroke are not yet mutable
  // Hint: 3-4 try creating a new stroke with the updated field value instead
  s.thickness = 5pt
}

--- field-feature-gated eval features() ---
// Error: 6-15 cannot access field `data-cell` because the `a11y-extras` feature is not enabled
// Hint: 6-15 try enabling the `a11y-extras` feature
// Hint: 6-15 see https://typst.app/help/compiler-features for more details
#pdf.data-cell

--- field-call-feature-gated eval features() ---
// Error: 6-15 cannot access field `data-cell` because the `a11y-extras` feature is not enabled
// Hint: 6-15 try enabling the `a11y-extras` feature
// Hint: 6-15 see https://typst.app/help/compiler-features for more details
#pdf.data-cell()
//@@ file tests/suite/scripting_for.typ
// Test for loops.

--- for-loop-basic paged ---

// Empty array.
#for x in () [Nope]

// Dictionary is traversed in insertion order.
// Should output `Name: Typst. Age: 2.`.
#for (k, v) in (Name: "Typst", Age: 2) [
  #k: #v.
]

// Block body.
// Should output `[1st, 2nd, 3rd, 4th]`.
#{
  "["
  for v in (1, 2, 3, 4) {
    if v > 1 [, ]
    [#v]
    if v == 1 [st]
    if v == 2 [nd]
    if v == 3 [rd]
    if v >= 4 [th]
   }
   "]"
}

// Content block body.
// Should output `2345`.
#for v in (1, 2, 3, 4, 5, 6, 7) [#if v >= 2 and v <= 5 { repr(v) }]

// Map captured arguments.
#let f1(..args) = args.pos().map(repr)
#let f2(..args) = args.named().pairs().map(p => repr(p.first()) + ": " + repr(p.last()))
#let f(..args) = (f1(..args) + f2(..args)).join(", ")
#f(1, a: 2)

--- for-loop-integrated eval ---
#let out = ()

// Values of array.
#for v in (1, 2, 3) {
  out += (v,)
}

// Indices and values of array.
#for (i, v) in ("1", "2", "3").enumerate() {
  test(repr(i + 1), v)
}

// Pairs of dictionary.
#for v in (a: 4, b: 5) {
  out += (v,)
}

// Keys and values of dictionary.
#for (k, v) in (a: 6, b: 7) {
  out += (k,)
  out += (v,)
}

#test(out, (1, 2, 3, ("a", 4), ("b", 5), "a", 6, "b", 7))

// Grapheme clusters of string.
#let first = true
#let joined = for c in "abc👩‍👩‍👦‍👦" {
  if not first { ", " }
  first = false
  c
}

#test(joined, "a, b, c, 👩‍👩‍👦‍👦")

// Return value.
#test(for v in "" [], none)
#test(type(for v in "1" []), content)

--- for-loop-over-bool eval ---
// Uniterable expression.
// Error: 11-15 cannot loop over boolean
#for v in true {}

--- for-loop-over-string eval ---
// Keys and values of strings.
// Error: 6-12 cannot destructure values of string
#for (k, v) in "hi" {
  dont-care
}

--- for-loop-destructuring-without-parentheses eval ---
// Destructuring without parentheses.
// Error: 7-8 unexpected comma
// Hint: 7-8 destructuring patterns must be wrapped in parentheses
#for k, v in (a: 4, b: 5) {
  dont-care
}

--- for-loop-destructuring-half eval ---
// Error: 7-8 unexpected comma
// Hint: 7-8 destructuring patterns must be wrapped in parentheses
#for k, in () {}

--- for-loop-incomplete eval ---
// Error: 5 expected pattern
#for

// Error: 5 expected pattern
#for//

// Error: 6 expected pattern
#{for}

// Error: 7 expected keyword `in`
#for v

// Error: 10 expected expression
#for v in

// Error: 15 expected block
#for v in iter

// Error: 5 expected pattern
#for
v in iter {}

// Error: 7-10 expected pattern, found string
// Error: 16 expected block
A#for "v" thing

// Error: 6-9 expected pattern, found string
#for "v" in iter {}

// Error: 7 expected keyword `in`
#for a + b in iter {}

// Error: 1:8-4:1 unclosed string
#for x "a
b
c
//@@ file tests/suite/scripting_get-rule.typ
--- get-rule-basic paged empty ---
// Test basic get rule.
#context test(text.lang, "en")
#set text(lang: "de")
#context test(text.lang, "de")
#text(lang: "es", context test(text.lang, "es"))

--- get-rule-in-function paged empty ---
// Test whether context is retained in nested function.
#let translate(..args) = args.named().at(text.lang)
#set text(lang: "de")
#context test(translate(de: "Inhalt", en: "Contents"), "Inhalt")

--- get-rule-call paged empty ---
// Test calling a contextual field as a function. Note that we need to wrap the
// field-access in parens to avoid an error for a disallowed field call.
#set heading(numbering: (..n) => n)
#context test((heading.numbering)(1, 2), arguments(1, 2))

--- get-rule-in-array-callback paged empty ---
// Test whether context is retained in built-in callback.
#set text(lang: "de")
#context test(
  ("en", "de", "fr").sorted(key: v => v != text.lang),
  ("de", "en", "fr"),
)

--- get-rule-folding paged empty ---
// Test folding.
#set rect(stroke: red)
#context {
  test(type(rect.stroke), stroke)
  test(rect.stroke.paint, red)
}
#[
  #set rect(stroke: 4pt)
  #context test(rect.stroke, 4pt + red)
]
#context test(rect.stroke, stroke(red))

--- get-rule-figure-caption-collision paged empty ---
// We have one collision: `figure.caption` could be both the element and a get
// rule for the `caption` field, which is settable. We always prefer the
// element. It's unfortunate, but probably nobody writes
// `set figure(caption: ..)` anyway.
#test(type(figure.caption), function)
#context test(type(figure.caption), function)

--- get-rule-assertion-failure paged ---
// Error: 10-31 Assertion failed: "en" != "de"
#context test(text.lang, "de")

--- get-rule-unknown-field paged ---
// Error: 15-20 function `text` does not contain field `langs`
#context text.langs

--- get-rule-inherent-field paged ---
// Error: 18-22 function `heading` does not contain field `body`
#context heading.body

--- get-rule-missing-context-no-context eval ---
// Error: 7-11 can only be used when context is known
// Hint: 7-11 try wrapping this in a `context` expression
// Hint: 7-11 the `context` expression should wrap everything that depends on this function
#text.lang

--- get-rule-unknown-field-no-context eval ---
// Error: 7-12 function `text` does not contain field `langs`
#text.langs

--- get-rule-inherent-field-no-context eval ---
// Error: 10-14 function `heading` does not contain field `body`
#heading.body
//@@ file tests/suite/scripting_if.typ
// Test if-else expressions.

--- if-markup paged ---
// Test condition evaluation.
#if 1 < 2 [
  One.
]

#if true == false [
  {Bad}, but we {dont-care}!
]

--- if-condition-complex paged ---
// Braced condition.
#if {true} [
  One.
]

// Content block in condition.
#if [] != none [
  Two.
]

// Multi-line condition with parens.
#if (
  1 + 1
    == 1
) [
  Nope.
] else {
  "Three."
}

// Multiline.
#if false [
  Bad.
] else {
  let point = "."
  "Four" + point
}

// Content block can be argument or body depending on whitespace.
#{
  if content == type[b] [Fi] else [Nope]
  if content == type [Nope] else [ve.]
}

#let i = 3
#if i < 2 [
  Five.
] else if i < 4 [
  Six.
] else [
  Seven.
]

--- if-else-if-else eval ---
// Test else if.

#let nth(n) = {
  str(n)
  if n == 1 { "st" }
  else if n == 2 { "nd" }
  else if n == 3 { "rd" }
  else { "th" }
}

#test(nth(1), "1st")
#test(nth(2), "2nd")
#test(nth(3), "3rd")
#test(nth(4), "4th")
#test(nth(5), "5th")

--- if-expression eval ---
// Value of if expressions.

#{
  let x = 1
  let y = 2
  let z

  // Returns if branch.
  z = if x < y { "ok" }
  test(z, "ok")

  // Returns else branch.
  z = if x > y { "bad" } else { "ok" }
  test(z, "ok")

  // Missing else evaluates to none.
  z = if x > y { "bad" }
  test(z, none)
}

--- if-condition-string-invalid eval ---
// Condition must be boolean.
// If it isn't, neither branch is evaluated.
// Error: 5-14 expected boolean, found string
#if "a" + "b" { nope } else { nope }

--- if-condition-invalid-and-wrong-type eval ---
// Make sure that we don't complain twice.
// Error: 5-12 cannot add integer and string
#if 1 + "2" {}

--- if-incomplete eval ---
// Error: 4 expected expression
#if

// Error: 5 expected expression
#{if}

// Error: 6 expected block
#if x

// Error: 2-6 unexpected keyword `else`
#else {}

// Should output `x`.
// Error: 4 expected expression
#if
x {}

// Should output `something`.
// Error: 6 expected block
#if x something

// Should output `A thing.`
// Error: 19 expected block
A#if false {} else thing

#if a []else [b]
#if a [] else [b]
#if a {} else [b]
//@@ file tests/suite/scripting_import.typ
// Test function and module imports.

--- import-basic paged ---
// Test basic syntax and semantics.

// Test that this will be overwritten.
#let value = [foo]

// Import multiple things.
#import "module.typ": fn, value
#fn[Like and Subscribe!]
#value

// Should output `bye`.
// Stop at semicolon.
#import "module.typ": a, c;bye

--- import-item-markup eval ---
// An item import.
#import "module.typ": item
#test(item(1, 2), 3)

--- import-item-in-code eval ---
// Code mode
#{
  import "module.typ": b
  test(b, 1)
}

--- import-wildcard-in-markup eval ---
// A wildcard import.
#import "module.typ": *

// It exists now!
#test(d, 3)

--- import-item-renamed eval ---
// A renamed item import.
#import "module.typ": item as something
#test(something(1, 2), 3)

--- import-items-renamed-mixed eval ---
// Mixing renamed and not renamed items.
#import "module.typ": fn, b as val, item as other
#test(val, 1)
#test(other(1, 2), 3)

--- import-nested-item eval ---
// Nested item imports.
#import "modules/chap1.typ" as orig-chap1
#import "modules/chap2.typ" as orig-chap2
#import "module.typ": chap2, chap2.name, chap2.chap1, chap2.chap1.name as othername
#test(chap2, orig-chap2)
#test(chap1, orig-chap1)
#test(name, "Peter")
#test(othername, "Klaus")

--- import-items-parenthesized eval ---
#import "module.typ": ()
#import "module.typ": (a)
#import "module.typ": (a, b)
#import "module.typ": (a, b, c, d)

#test(a, none)
#test(b, 1)
#test(c, 2)
#test(d, 3)

--- import-items-parenthesized-multiline eval ---
#import "module.typ": (
  a
)
#import "module.typ": (
  a, b as e,
  c,


      d,
)

#test(a, none)
#test(e, 1)
#test(c, 2)
#test(d, 3)

--- import-items-parenthesized-invalid eval ---
// Error: 23-24 unclosed delimiter
#import "module.typ": (a, b, c

--- import-items-parenthesized-invalid-2 eval ---
// Error: 23-24 unclosed delimiter
#import "module.typ": (

--- import-items-parenthesized-invalid-3 eval ---
// Error: 23-24 unclosed delimiter
#import "module.typ": (
  a, b,
  c,


--- import-from-function-scope paged ---
// Test importing from function scopes.

#import enum: item
#import assert.with(true): *

#enum(
   item(1)[First],
   item(5)[Fifth]
)
#eq(10, 10)
#ne(5, 6)

--- import-from-function-scope-item-renamed eval ---
// Test renaming items imported from function scopes.
#import assert: eq as aseq
#aseq(10, 10)

--- import-from-function-scope-nested-import eval ---
// Test importing items from function scopes via nested import.
#import std: grid.cell, table.cell as tcell
#test(cell, grid.cell)
#test(tcell, table.cell)

--- import-from-type-scope eval ---
// Test importing from a type's scope.
#import array: zip
#test(zip((1, 2), (3, 4)), ((1, 3), (2, 4)))

--- import-from-type-scope-item-renamed eval ---
// Test importing from a type's scope with renaming.
#import array: pop as renamed-pop
#test(renamed-pop((1, 2)), 2)

--- import-from-type-scope-nested-import eval ---
// Test importing from a type's scope with nested import.
#import std: array.zip, array.pop as renamed-pop
#test(zip((1, 2), (3, 4)), ((1, 3), (2, 4)))
#test(renamed-pop((1, 2)), 2)

--- import-from-file-bare eval ---
// A module import without items.
#import "module.typ"
#test(module.b, 1)
#test(module.item(1, 2), 3)
#test(module.push(2), 3)

--- import-from-file-bare-invalid eval ---
// Error: 9-33 module name would not be a valid identifier
// Hint: 9-33 you can rename the import with `as`
#import "modules/with space.typ"

--- import-from-file-bare-dynamic eval ---
// Error: 9-26 dynamic import requires an explicit name
// Hint: 9-26 you can name the import with `as`
#import "mod" + "ule.typ"

--- import-from-path-bare eval ---
// Error: 9-27 dynamic import requires an explicit name
// Hint: 9-27 you can name the import with `as`
#import path("module.typ")

--- import-from-str-var-bare eval ---
#let p = "module.typ"
// Error: 9-10 dynamic import requires an explicit name
// Hint: 9-10 you can name the import with `as`
#import p

--- import-from-path-var-bare eval ---
#let p = path("module.typ")
// Error: 9-10 dynamic import requires an explicit name
// Hint: 9-10 you can name the import with `as`
#import p

--- import-from-dict-field-bare eval ---
#let d = (p: "module.typ")
// Error: 9-12 dynamic import requires an explicit name
// Hint: 9-12 you can name the import with `as`
#import d.p

--- import-from-file-renamed-dynamic eval ---
#import "mod" + "ule.typ" as mod
#test(mod.b, 1)

--- import-from-file-renamed eval ---
// A renamed module import without items.
#import "module.typ" as other
#test(other.b, 1)
#test(other.item(1, 2), 3)
#test(other.push(2), 3)

--- import-from-file-items-renamed-mixed eval ---
// Mixing renamed module and items.
#import "module.typ" as newname: b as newval, item
#test(newname.b, 1)
#test(newval, 1)
#test(item(1, 2), 3)
#test(newname.item(1, 2), 3)

--- import-from-str-var-renamed eval ---
#let p = "module.typ"
#import p as f
#test(f.b, 1)

--- import-from-path-renamed eval ---
#import path("modules/chap1.typ") as chap1
#import chap1.module-path as f
#test(f.b, 1)

--- import-from-function-scope-bare eval ---
// Warning: 9-13 this import has no effect
#import enum

--- import-from-function-scope-renamed eval ---
// Renamed module import with function scopes.
#import enum as othernum
#test(enum, othernum)

--- import-from-function-scope-renamed-twice eval ---
// Mixing renamed module import from function with renamed item import.
#import assert as asrt
#import asrt: ne as asne
#asne(1, 2)

--- import-from-module-bare eval ---
#import "modules/chap1.typ" as mymod
// Warning: 9-14 this import has no effect
#import mymod
// The name `chap1` is not bound.
// Error: 2-7 unknown variable `chap1`
#chap1

--- import-module-nested eval ---
#import std.calc: pi
#test(pi, calc.pi)

--- import-module-nested-bare eval ---
#import "module.typ"
#import module.chap2
#test(chap2.name, "Peter")

--- import-no-whitespace eval ---
// Who needs whitespace anyways?
#import"module.typ":*

--- import-trailing-comma eval ---
// Allow the trailing comma.
#import "module.typ": a, c,

--- import-source-field-access paged ---
// Usual importing syntax also works for function scopes
#let d = (e: enum)
#import d.e
#import d.e as renamed
#import d.e: item
#item(2)[a]

--- import-item-rename-unnecessary eval ---
// Warning: 23-27 unnecessary import rename to same name
#import enum: item as item

--- import-rename-unnecessary eval ---
// Warning: 17-21 unnecessary import rename to same name
#import enum as enum

--- import-rename-necessary eval ---
#import "module.typ" as module: a
#test(module.a, a)

--- import-rename-unnecessary-mixed eval ---
// Warning: 17-21 unnecessary import rename to same name
#import enum as enum: item

// Warning: 17-21 unnecessary import rename to same name
// Warning: 31-35 unnecessary import rename to same name
#import enum as enum: item as item

--- import-item-rename-unnecessary-but-ok eval ---
#import "modul" + "e.typ" as module
#test(module.b, 1)

--- import-from-closure-invalid eval ---
// Can't import from closures.
#let f(x) = x
// Error: 9-10 cannot import from user-defined functions
#import f: x

--- import-from-closure-renamed-invalid eval ---
// Can't import from closures, despite renaming.
#let f(x) = x
// Error: 9-10 cannot import from user-defined functions
#import f as g

--- import-from-with-closure-invalid eval ---
// Can't import from closures, despite modifiers.
#let f(x) = x
// Error: 9-18 cannot import from user-defined functions
#import f.with(5): x

--- import-from-with-closure-literal-invalid eval ---
// Error: 9-18 cannot import from user-defined functions
#import () => {5}: x

--- import-from-int-invalid eval ---
// Error: 9-10 expected path, module, function, or type, found integer
#import 5: something

--- import-from-int-renamed-invalid eval ---
// Error: 9-10 expected path, module, function, or type, found integer
#import 5 as x

--- import-from-string-invalid eval ---
// Error: 9-11 failed to load file (is a directory)
#import "": name

--- import-from-string-renamed-invalid eval ---
// Error: 9-11 failed to load file (is a directory)
#import "" as x

--- import-file-not-found-invalid eval ---
// Error: 9-20 file not found (searched at tests/suite/scripting/lib/0.2.1)
#import "lib/0.2.1"

--- import-file-not-found-renamed-invalid eval ---
// Error: 9-20 file not found (searched at tests/suite/scripting/lib/0.2.1)
#import "lib/0.2.1" as x

--- import-file-not-valid-utf-8 eval ---
// Some non-text stuff.
// Error: 9-35 file is not valid UTF-8
#import "/assets/images/rhino.png"

--- import-item-not-found eval ---
// Unresolved import.
// Error: 23-35 unresolved import
#import "module.typ": non_existing

--- import-cyclic eval ---
// Cyclic import of this very file.
// Error: 9-23 cyclic import
#import "./import.typ"

--- import-cyclic-in-other-file eval ---
// Cyclic import in other file.
// Error: "tests/suite/scripting/modules/cycle2.typ" 2:9-2:21 cyclic import
#import "./modules/cycle1.typ": *

This is never reached.

--- import-renamed-old-name eval ---
// Renaming does not import the old name (without items).
#import "./modules/chap1.typ" as something
#test(something.name, "Klaus")
// Error: 7-12 unknown variable `chap1`
#test(chap1.name, "Klaus")

--- import-items-renamed-old-name eval ---
// Renaming does not import the old name (with items).
#import "./modules/chap1.typ" as something: name as other
#test(other, "Klaus")
#test(something.name, "Klaus")
// Error: 7-12 unknown variable `chap1`
#test(chap1.b, "Klaus")

--- import-nested-invalid-type eval ---
// Error: 19-21 expected module, function, or type, found float
#import std: calc.pi.something

--- import-incomplete eval ---
// Error: 8 expected expression
#import

--- import-item-string-invalid eval ---
// Error: 26-29 unexpected string
#import "module.typ": a, "b", c

--- import-bad-token eval ---
// Error: 23-24 unexpected equals sign
#import "module.typ": =

--- import-duplicate-comma eval ---
// An additional trailing comma.
// Error: 31-32 unexpected comma
#import "module.typ": a, b, c,,

--- import-no-colon eval ---
// Error: 2:2 expected semicolon or line break
#import "module.typ
"stuff

--- import-bad-token-star eval ---
// A star in the list.
// Error: 26-27 unexpected star
#import "module.typ": a, *, b

--- import-item-after-star eval ---
// An item after a star.
// Error: 24 expected semicolon or line break
#import "module.typ": *, a

--- import-bad-colon-in-items eval ---
// Error: 14-15 unexpected colon
// Error: 16-17 unexpected integer
#import "": a: 1

--- import-incomplete-nested eval ---
// Error: 15 expected identifier
#import "": a.

--- import-wildcard-in-nested eval ---
// Error: 15 expected identifier
// Error: 15-16 unexpected star
#import "": a.*

--- import-missing-comma eval ---
// Error: 14 expected comma
#import "": a b

--- import-from-package-bare eval ---
// Test import without items.
#import "@test/adder:0.1.0"
#test(adder.add(2, 8), 10)

--- import-from-package-dynamic eval ---
// Error: 9-33 dynamic import requires an explicit name
// Hint: 9-33 you can name the import with `as`
#import "@test/" + "adder:0.1.0"

--- import-from-package-renamed-dynamic eval ---
#import "@test/" + "adder:0.1.0" as adder
#test(adder.add(2, 8), 10)

--- import-from-package-items eval ---
// Test import with items.
#import "@test/adder:0.1.0": add
#test(add(2, 8), 10)

--- import-from-package-required-compiler-version eval ---
// Test too high required compiler version.
// Error: 9-29 package requires Typst 1.0.0 or newer (current version is VERSION)
#import "@test/future:0.1.0": future

--- import-from-package-namespace-invalid-1 eval ---
// Error: 9-13 `@` is not a valid package namespace
#import "@@": *

--- import-from-package-name-missing-1 eval ---
// Error: 9-16 package specification is missing name
#import "@heya": *

--- import-from-package-namespace-invalid-2 eval ---
// Error: 9-15 `123` is not a valid package namespace
#import "@123": *

--- import-from-package-name-missing-2 eval ---
// Error: 9-17 package specification is missing name
#import "@test/": *

--- import-from-package-version-missing-1 eval ---
// Error: 9-22 package specification is missing version
#import "@test/mypkg": *

--- import-from-package-name-invalid eval ---
// Error: 9-20 `$$$` is not a valid package name
#import "@test/$$$": *

--- import-from-package-version-missing-2 eval ---
// Error: 9-23 package specification is missing version
#import "@test/mypkg:": *

--- import-from-package-version-missing-minor eval ---
// Error: 9-24 version number is missing minor version
#import "@test/mypkg:0": *

--- import-from-package-version-major-invalid-1 eval ---
// Error: 9-29 `latest` is not a valid major version
#import "@test/mypkg:latest": *

--- import-from-package-version-major-invalid-2 eval ---
// Error: 9-29 `-3` is not a valid major version
#import "@test/mypkg:-3.0.0": *

--- import-from-package-version-missing-patch-1 eval ---
// Error: 9-26 version number is missing patch version
#import "@test/mypkg:0.3": *

--- import-from-package-version-missing-patch-2 eval ---
// Error: 9-27 version number is missing patch version
#import "@test/mypkg:0.3.": *

--- import-from-file-package-lookalike eval ---
// Error: 9-28 file not found (searched at tests/suite/scripting/#test/mypkg:1.0.0)
#import "#test/mypkg:1.0.0": *

--- import-feature-gated-item eval features() ---
// Error: 14-18 cannot import `html` because the `html` feature is not enabled
// Hint: 14-18 try enabling the `html` feature
// Hint: 14-18 see https://typst.app/help/compiler-features for more details
#import std: html

--- import-feature-gated-field eval features() ---
// Error: 18-31 cannot import `table-summary` because the `a11y-extras` feature is not enabled
// Hint: 18-31 try enabling the `a11y-extras` feature
// Hint: 18-31 see https://typst.app/help/compiler-features for more details
#import std: pdf.table-summary

--- import-deprecated-item eval ---
// Warning: 15-25 this function is useless
#import test: deprecated
// Warning: 16-26 this value is useless
#import check: deprecated

--- import-feature-gated-value eval features() ---
// Error: 16-21 cannot import `gated` because the `html` feature is not enabled
// Hint: 16-21 try enabling the `html` feature
// Hint: 16-21 see https://typst.app/help/compiler-features for more details
#import check: gated

--- import-feature-gated-field-of-type eval features() ---
// Error: 15-20 cannot import `gated` because the `html` feature is not enabled
// Hint: 15-20 try enabling the `html` feature
// Hint: 15-20 see https://typst.app/help/compiler-features for more details
#import test: gated

--- import-feature-gated-wildcard-unused eval features() ---
#let func() = {
  import check: *
}

--- import-feature-gated-wildcard-shadowed eval features() ---
#let gated() = {}
#let func() = {
  import check: *
  gated()
}
#func()

--- import-feature-gated-wildcard-used eval features() ---
#let func() = {
  import check: *
  // Error: 3-8 unknown variable `gated`
  gated()
}
#func()

--- issue-7393-import-error-string-unclosed eval ---
// More dedicated tests for this are in `crates/typst-syntax/src/reparser.rs`
// Error: 1:9-3:1 unclosed string
#import "@preview/unify:0.1.0

--- issue-7393-import-error-string-unclosed-newlines eval ---
// Error: 1:9-5:1 unclosed string
#import "a
b
c

--- issue-7393-import-error-extra-quote eval ---
// Error: 1:31-4:1 unclosed string
#import "@preview/unify:0.1.0""a
b
c
//@@ file tests/suite/scripting_include.typ
// Test module includes.

--- include-file paged ---
#set page(width: 200pt)

// Include a file
#include "modules/chap1.typ"

// Expression as a file name.
#let chap2 = include "modu" + "les/chap" + "2.typ"

-- _Intermission_ --
#chap2

--- include-file-not-found eval ---
#{
  // Error: 19-38 file not found (searched at tests/suite/scripting/modules/chap3.typ)
  let x = include "modules/chap3.typ"
}

--- include-path paged ---
#import "modules/chap1.typ": chap2-path
#include chap2-path

--- include-path-like-str-not-found eval ---
#import "modules/chap1.typ": chap2-str
// Error: 10-19 file not found (searched at tests/suite/scripting/chap2.typ)
#include chap2-str

--- include-no-bindings eval ---
#include "modules/chap1.typ"

// The variables of the file should not appear in this scope.
// Error: 2-6 unknown variable `name`
#name

--- include-semicolon-or-linebreak eval ---
// Error: 18 expected semicolon or line break
#include "hi.typ" Hi
//@@ file tests/suite/scripting_let.typ
// Test let bindings.

--- let-basic paged ---
// Automatically initialized with none.
#let x
#test(x, none)

// Manually initialized with one.
#let z = 1
#test(z, 1)

// Syntax sugar for function definitions.
#let fill = conifer
#let f(body) = rect(width: 2cm, fill: fill, inset: 5pt, body)
#f[Hi!]

--- let-termination paged ---
// Termination.

// Terminated by line break.
#let v1 = 1
One

// Terminated by semicolon.
#let v2 = 2; Two

// Terminated by semicolon and line break.
#let v3 = 3;
Three

#test(v1, 1)
#test(v2, 2)
#test(v3, 3)

--- let-valid-idents eval ---
// Test what constitutes a valid Typst identifier.
#let name = 1
#test(name, 1)
#let name_ = 1
#test(name_, 1)
#let name-2 = 1
#test(name-2, 1)
#let name_2 = 1
#test(name_2, 1)
#let __name = 1
#test(__name, 1)
#let ůñıćóðė = 1
#test(ůñıćóðė, 1)

--- let-invalid-grapheme-in-ident eval ---
// Some combining marks, here U+20DD COMBINING ENCLOSING CIRCLE, can extend
// grapheme clusters, but are not in `xid_continue`.
// Error: 7-8 the character `⃝` is not valid in code
#let A⃝BC = 5

--- let-binding-keyword-in-markup eval ---
// Error: 6-8 expected pattern, found keyword `as`
// Hint: 6-8 keyword `as` is not allowed as an identifier; try `as_` instead
#let as = 1 + 2

--- let-binding-keyword-in-code eval ---
#{
  // Error: 7-9 expected pattern, found keyword `as`
  // Hint: 7-9 keyword `as` is not allowed as an identifier; try `as_` instead
  let as = 10
}

--- let-ident-parenthesized eval ---
// Test parenthesised assignments.
#let (a) = (1, 2)

--- let-incomplete eval ---
// Error: 5 expected pattern
#let

// Error: 6 expected pattern
#{let}

// Error: 6-9 expected pattern, found string
#let "v"

// Error: 7 expected semicolon or line break
#let v 1

// Error: 9 expected expression
#let v =

// Error: 6-9 expected pattern, found string
#let "v" = 1

// Terminated because expression ends.
// Error: 12 expected semicolon or line break
#let v4 = 4 Four

// Terminated by semicolon even though we are in a paren group.
// Error: 18 expected expression
// Error: 11-12 unclosed delimiter
#let v5 = (1, 2 + ; Five

// Error: 9-13 expected pattern, found boolean
#let (..true) = false

--- underscore-invalid eval ---
#let _ = 4

#for _ in range(2) []

// Error: 2-3 unexpected underscore
#_

// Error: 8-9 expected expression, found underscore
#lorem(_)

// Error: 3-4 expected expression, found underscore
#(_,)

// Error: 3-4 expected expression, found underscore
#{_}

// Error: 8-9 expected expression, found underscore
#{ 1 + _ }

--- let-function-incomplete eval ---
// Error: 13 expected equals sign
#let func(x)

// Error: 15 expected expression
#let func(x) =

--- let-function-parenthesized eval ---
// This is not yet parsed in the ideal way.
// Error: 12 expected equals sign
#let (func)(x)

--- let-function-parenthesized-with-init eval ---
// These errors aren't great.
// Error: 12 expected equals sign
// Error: 15-15 expected semicolon or line break
#let (func)(x) = 3

--- let-with-no-init-group eval ---
// This was unintentionally allowed ...
// Error: 9 expected equals sign
#let (a)

--- let-with-no-init-destructuring eval ---
// ... where this wasn't.
// Error: 12 expected equals sign
#let (a, b)

--- issue-4027-let-binding-with-keyword-context eval ---
// Error: 6-13 expected pattern, found keyword `context`
// Hint: 6-13 keyword `context` is not allowed as an identifier; try `context_` instead
#let context = 5

--- issue-4027-let-binding-with-keyword-let eval ---
// Error: 6-9 expected pattern, found keyword `let`
// Hint: 6-9 keyword `let` is not allowed as an identifier; try `let_` instead
#let let = 5

--- issue-4027-let-binding-with-destructured-keywords eval ---
// Error: 7-14 expected pattern, found keyword `context`
// Hint: 7-14 keyword `context` is not allowed as an identifier; try `context_` instead
// Error: 21-24 expected pattern, found keyword `let`
// Hint: 21-24 keyword `let` is not allowed as an identifier; try `let_` instead
#let (context, foo, let) = (5, 6, 7)
//@@ file tests/suite/scripting_loop.typ
// Test break and continue in loops.

--- loop-break-basic eval ---
// Test break.

#let var = 0
#let error = false

#for i in range(10) {
  var += i
  if i > 5 {
    break
    error = true
  }
}

#test(var, 21)
#test(error, false)

--- loop-break-join-basic eval ---
// Test joining with break.

#let i = 0
#let x = while true {
  i += 1
  str(i)
  if i >= 5 {
    "."
    break
  }
}

#test(x, "12345.")

--- loop-continue-basic eval ---
// Test continue.

#let i = 0
#let x = 0

#while x < 8 {
  i += 1
  if calc.rem(i, 3) == 0 {
    continue
  }
  x += i
}

// If continue did not work, this would equal 10.
#test(x, 12)

--- loop-continue-join eval ---
// Test joining with continue.

#let x = for i in range(5) {
  "a"
  if calc.rem(i, 3) == 0 {
    "_"
    continue
  }
  str(i)
}

#test(x, "a_a1a2a_a4")

--- loop-break-outside-of-loop eval ---
// Test break outside of loop.
#let f() = {
  // Error: 3-8 cannot break outside of loop
  break
}

#for i in range(1) {
  f()
}

--- loop-break-join-in-last-arg eval ---
// Test break in function call.
#let identity(x) = x
#let out = for i in range(5) {
  "A"
  identity({
    "B"
    break
  })
  "C"
}

#test(out, "AB")

--- loop-continue-outside-of-loop-in-block eval ---
// Test continue outside of loop.

// Error: 12-20 cannot continue outside of loop
#let x = { continue }

--- loop-continue-outside-of-loop-in-markup eval ---
// Error: 2-10 cannot continue outside of loop
#continue

--- loop-break-join-in-nested-blocks paged ---
// Should output `Hello World 🌎`.
#for _ in range(10) {
  [Hello ]
  [World #{
    [🌎]
    break
  }]
}

--- loop-break-join-set-and-show paged ---
// Should output `Some` in red, `Some` in blue and `Last` in green.
// Everything should be in smallcaps.
#for color in (red, blue, green, yellow) [
  #set text(font: "Roboto")
  #show: it => text(fill: color, it)
  #smallcaps(if color != green [
    Some
  ] else [
    Last
    #break
  ])
]

--- loop-break-join-in-set-rule-args paged ---
// Test break in set rule.
// Should output `Hi` in blue.
#for i in range(10) {
  [Hello]
  set text(blue, ..break)
  [Not happening]
}

--- loop-break-join-in-first-arg paged ---
// Test second block during break flow.

#for i in range(10) {
  table(
    { [A]; break },
    for _ in range(3) [B]
  )
}
//@@ file tests/suite/scripting_methods.typ
// Test field call syntax in code and math.

// A pattern used in some of the `field-call-accent-*` tests is to create a
// symbol with an accent character (`sym.tilde`) as a field. This lets us create
// a callable field with an arbitrary name without the inconvenience of
// creating a whole module.

--- field-call-whitespace eval ---
// We allow whitespace around the dot.
#test( "Hi there" . split() , ("Hi", "there"))

--- field-call-multiline eval ---
// Test multiline chain in code block.
#{
  let rewritten = "Hello. This is a sentence. And one more."
    .split(".")
    .map(s => s.trim())
    .filter(s => s != "")
    .map(s => s + "!")
    .join("\n ")

  test(rewritten, "Hello!\n This is a sentence!\n And one more!")
}

--- math-field-call-accent-accent eval ---
// Test math method call for an accent symbol.
#test($arrow.l.r(x)$, $#math.arrow.l.r[\x]$)

--- math-field-call-accent-non-func eval ---
// Test calling a symbol whose field isn't an accent.
#test($pi.alt(x)$, $pi.alt/**/(x)$)

--- field-call-import-module eval ---
#import "module.typ"
#test(module.item(1, 2), 3)

--- math-field-call-import-module eval ---
#import "module.typ"
#test($module.item(#1, #2)$, $#3$)

--- field-call-missing-field eval ---
// Error: 6-13 function `pdf` does not contain field `attache`
#pdf.attache()

--- math-field-call-missing-field eval ---
// Error: 14-19 function `assert` does not contain field `noteq`
$ std.assert.noteq() $

--- field-call-unknown-generic-type eval ---
// Error: 2-8 type array has no method `fun`
#().fun()

--- math-field-call-unknown-generic-type eval ---
// Error: 3-14 type color has no method `fun`
$ std.red.fun() $

--- field-call-unknown-content eval ---
// Error: 2-8 element sequence has no method `fun`
#[].fun()

--- math-field-call-unknown-content eval ---
#let pi = [pi]
// Error: 3-9 element text has no method `fun`
$ pi.fun() $

--- field-call-invalid-content-field-func eval ---
// This would be cursed to allow.
// Error: 2-48 `supplement` is not a valid method for element `heading`
// Hint: 2-48 to call the stored function, wrap the field access in parentheses: `(heading([], supplement: x => x + 1).supplement)(..)`
#heading([], supplement: x => x + 1).supplement(2)

--- math-field-call-invalid-content-field-func eval ---
#let cancel = math.cancel($x$, angle: ang => ang + 90deg)
// Error: 3-15 `angle` is not a valid method for element `cancel`
// Hint: 3-15 to call the stored function, use code mode and wrap the field access in parentheses: `#(cancel.angle)(..)`
$ cancel.angle(#30deg) $

--- field-call-invalid-non-func eval ---
// Error: 2-26 `stroke` is not a valid method for element `line`
// Hint: 2-26 to access the `stroke` field, remove the function arguments: `line(stroke: red).stroke`
#line(stroke: red).stroke()

--- math-field-call-invalid-non-func eval ---
// Error: 27-36 `y` is not a valid method for type `alignment`
// Hint: 27-36 try adding a space before the parentheses
#test($std.top.y/**/()$, $std.top.y()$)
// This is potentially worth allowing in the future.

--- field-call-dict-invalid eval ---
// Test attempting to call a function from a dictionary.
// Error: 2-34 cannot directly call dictionary keys as functions
// Hint: 2-34 to call the stored function, wrap the field access in parentheses: `((call-me: () => "maybe").call-me)(..)`
// Hint: 2-34 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
#(call-me: () => "maybe").call-me()

--- math-field-call-dict-invalid eval ---
#let pi = (alt: _ => math.pi.alt)
// Error: 3-9 cannot directly call dictionary keys as functions
// Hint: 3-9 to call the stored function, use code mode and wrap the field access in parentheses: `#(pi.alt)(..)`
// Hint: 3-9 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
$ pi.alt() $

--- field-call-dict-non-func eval ---
// Error: 2-24 cannot directly call dictionary keys as functions
// Hint: 2-24 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
// Hint: 2-24 to access the `non-func` key, remove the function arguments: `(non-func: 1).non-func`
#(non-func: 1).non-func()

--- math-field-call-dict-non-func eval ---
// The hint should differ slightly to account for being in math mode.
#let pi = (alt: math.pi.alt)
// Error: 3-9 cannot directly call dictionary keys as functions
// Hint: 3-9 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
// Hint: 3-9 try adding a space before the parentheses
$ pi.alt() $

--- field-call-args-invalid eval ---
// Test attempting to call a function from a named argument field.
// Error: 2-43 cannot directly call named argument fields as functions
// Hint: 2-43 to call the stored function, wrap the field access in parentheses: `(arguments(call-me: () => "maybe").call-me)(..)`
// Hint: 2-43 named arguments cannot be used with method syntax as argument names could conflict with built-in method names
#arguments(call-me: () => "maybe").call-me()

--- math-field-call-args-invalid eval ---
#let pi = arguments(alt: _ => math.pi.alt)
// Error: 3-9 cannot directly call named argument fields as functions
// Hint: 3-9 to call the stored function, use code mode and wrap the field access in parentheses: `#(pi.alt)(..)`
// Hint: 3-9 named arguments cannot be used with method syntax as argument names could conflict with built-in method names
$ pi.alt() $

--- field-call-args-non-func eval ---
// Error: 2-33 cannot directly call named argument fields as functions
// Hint: 2-33 to access the `non-func` argument, remove the function arguments: `arguments(non-func: 1).non-func`
// Hint: 2-33 named arguments cannot be used with method syntax as argument names could conflict with built-in method names
#arguments(non-func: 1).non-func()

--- math-field-call-args-non-func eval ---
// The hint should differ slightly to account for being in math mode.
#let pi = arguments(alt: math.pi.alt)
// Error: 3-9 cannot directly call named argument fields as functions
// Hint: 3-9 try adding a space before the parentheses
// Hint: 3-9 named arguments cannot be used with method syntax as argument names could conflict with built-in method names
$ pi.alt() $

--- field-call-no-context eval ---
#set heading(numbering: (..n) => none)
// Error: 10-19 can only be used when context is known
// Hint: 10-19 try wrapping this in a `context` expression
// Hint: 10-19 the `context` expression should wrap everything that depends on this function
#heading.numbering(1)

--- math-field-call-no-context eval ---
#set heading(numbering: (..n) => none)
// Error: 14-23 can only be used when context is known
// Hint: 14-23 try wrapping this in a `context` expression
// Hint: 14-23 the `context` expression should wrap everything that depends on this function
$std.heading.numbering(#1)$

--- field-call-contextual-invalid paged empty ---
// Test calling a contextual field as a function.
#set heading(numbering: (..n) => none)
// Error: 10-27 `numbering` is not a valid method for type `function`
// Hint: 10-27 to call the stored function, wrap the field access in parentheses: `(heading.numbering)(..)`
#context heading.numbering(1)

--- math-field-call-contextual-invalid paged empty ---
#set heading(numbering: (..n) => none)
// Error: 11-32 `numbering` is not a valid method for type `function`
// Hint: 11-32 to call the stored function, use code mode and wrap the field access in parentheses: `#(std.heading.numbering)(..)`
#context $std.heading.numbering(#1)$

--- field-call-contextual-non-func paged empty ---
#set heading(numbering: "1.")
// Error: 10-27 `numbering` is not a valid method for type `function`
// Hint: 10-27 to access the `numbering` field, remove the function arguments: `heading.numbering`
#context heading.numbering(1)

--- math-field-call-contextual-non-func paged empty ---
#set heading(numbering: "1.")
// Error: 11-32 `numbering` is not a valid method for type `function`
// Hint: 11-32 try adding a space before the parentheses
#context $std.heading.numbering(#1)$

--- field-call-mut-basic eval ---
// Mutating methods mutate a variable.
#let numbers = (1, 2, 3)
#test(numbers.remove(1), 2)
#test(numbers, (1, 3))

--- math-field-call-mut-basic eval ---
// Test mutating method calls in math.
// Currently they just error, but may be allowed in the future.
#let array = (1, 2)
#let dict = (one: 1)
#let _ = $
  // Error: 3-20 cannot call mutating methods in math
  // Hint: 3-20 try using code mode to call the method: `#array.push("two")`
  array.push("two")
  array.insert(#2, dict.remove("one"))
  dict.insert(array.pop(), array.remove(#1))
$
#test(array, (1, 1))
#test(dict, (two: 2))

--- field-call-mut-temporary eval ---
#let numbers = (1, 2, 3)
// Error: 2-43 cannot mutate a temporary value
#numbers.map(v => v / 2).sorted().map(str).remove(4)

--- field-call-mut-str-accessor eval ---
// Some methods return new values rather than mutable references, so they can't
// be used as assignment targets.
#{
  let s = "string"
  // Error: 3-12 cannot mutate a temporary value
  s.first() = "S"
}

--- field-call-mut-bytes-accessor eval ---
#{
  let b = bytes("ab")
  // Error: 3-10 cannot mutate a temporary value
  b.at(0) = 5
}

--- field-call-mut-version-accessor eval ---
#{
  let v = version(1, 2, 3)
  // Error: 3-10 cannot mutate a temporary value
  v.at(0) = 5
}

--- field-call-mut-content-accessor eval ---
#{
  let c = [hi]
  // Error: 3-15 cannot mutate a temporary value
  c.at("body") = [bye]
}

--- field-call-mut-args-accessor eval ---
#{
  let a = arguments(1, 2)
  // Error: 3-10 cannot mutate a temporary value
  a.at(0) = 5
}

--- field-call-mut-constant eval ---
// Error: 2-5 cannot mutate a constant: box
#box.push(1)

--- field-call-dict-pop eval ---
// `pop` is a mutating method on arrays, but not dictionaries.
#let dict = (pop: () => none)
// Error: 2-10 cannot directly call dictionary keys as functions
// Hint: 2-10 to call the stored function, wrap the field access in parentheses: `(dict.pop)(..)`
// Hint: 2-10 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
#dict.pop()

--- field-call-dict-non-func-pop eval ---
#let dict = (pop: none)
// Error: 2-10 cannot directly call dictionary keys as functions
// Hint: 2-10 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
// Hint: 2-10 to access the `pop` key, remove the function arguments: `dict.pop`
#dict.pop()

--- math-field-call-dict-non-func-pop eval ---
#let dict = (pop: none)
// Error: 3-13 cannot call mutating methods in math
// Hint: 3-13 try using code mode to call the method: `#dict.pop()`
$ dict.pop() $

--- field-call-dict-non-func-pop-arg eval ---
#let dict = (pop: none)
// Error: 2-10 cannot directly call dictionary keys as functions
// Hint: 2-10 dictionary keys cannot be used with method syntax as keys could conflict with built-in method names
// Hint: 2-10 to access the `pop` key, remove the function arguments: `dict.pop`
#dict.pop(arg: "something")

--- math-field-call-dict-non-func-pop-arg eval ---
#let dict = (pop: none)
// Error: 3-29 cannot call mutating methods in math
// Hint: 3-29 try using code mode to call the method: `#dict.pop(arg: "something")`
$ dict.pop(arg: "something") $

--- field-call-mut-access eval ---
// Test calling a mutating method from accessor methods.
#{
  let matrix = (((1,), (2,)), ((3,), (4,)))
  matrix.at(1).at(0).push(5)
  test(matrix, (((1,), (2,)), ((3, 5), (4,))))
}

--- field-call-mut-access-module-push eval ---
// Edge case for module access that isn't fixed.
#import "module.typ"
// Works because the method name isn't categorized as mutating.
#test((module,).at(0).item(1, 2), 3)
// Doesn't work because of mutating name.
// Error: 7-16 cannot mutate a temporary value
#test((module,).at(0).push(2), 3)

--- math-field-call-mut-access-module-push eval ---
// This used to error, but math now correctly ignores the mutable method name.
#import "module.typ"
#let indirect = (mod: module)
#test($indirect.mod.push(#2)$, $#3$)

--- field-call-mut-eval-order eval ---
// Test evaluation order of mutating methods with accessors.
#{
  let pair = (1, 2)
  let arrays = ((), (), ())
  arrays.at(pair.remove(0)).push(pair.remove(0))
  //             ^^^^^^ second (2)    ^^^^^^ first (1)
  test(arrays, ((), (), (1,)))
}

--- field-call-mut-eval-order-self eval ---
// Test evaluation order when arguments mutate the variable itself.
#{
  let pair = ((0,), (1,))
  // Error: 3-13 array index out of bounds (index: 1, len: 1)
  pair.at(1).insert(0, pair.remove(0))
}

--- field-call-mut-eval-order-assign eval ---
// Test evaluation order when assigning to a variable in an argument.
#{
  let what = ()
  what.insert("what", what = (:))
  test(what, (what: none))
}

--- field-call-mut-eval-order-replace eval ---
// Test whether replacing a field while accessing it causes an error.
#{
  let dict = (one: ())
  dict.one.insert("two", dict.insert("one", (:)))
  test(dict.one, (two: none))
}

--- field-call-mut-eval-order-replace-nested eval ---
// Test whether replacing a nested field while accessing it causes an error.
#{
  let dict = (one: (two: ()))
  dict.one.two.insert("three", dict.insert("one", (two: (:))))
  test(dict.one, (two: (three: none)))
}

--- field-call-accent-eval-order-assign eval ---
// Test evaluation order when assigning to a variable in an argument.
#{
  let sm = symbol("m", ("method", sym.tilde))
  let array = ()
  // Error: 8-20 type array has no method `method`
  test(array.method(array = sm), sym.tilde(none))
  test(array, sm)
}

--- math-field-call-accent-eval-order-assign eval ---
// Test evaluation order when assigning to a variable in an argument in math.
#{
  let sm = symbol("m", ("method", sym.tilde))
  let array = ()
  // Error: 9-21 type array has no method `method`
  test($array.method(#{array = sm})$, $#sym.tilde(none)$)
  test(array, sm)
}

--- field-call-accent-eval-order-shadowed eval ---
// Test shadowing a variable in arguments while calling a method on it.
#{
  let sm = symbol("m", ("method", sym.tilde))
  test(sm.method(let sm = false), sym.tilde(none))
  test(sm, false)
}

--- math-field-call-accent-eval-order-shadowed eval ---
// Test shadowing a variable in arguments while calling a method on it in math.
#{
  let sm = symbol("m", ("method", sym.tilde))
  test($sm.method(#let sm = false;)$, $#sym.tilde(none)$)
  test(sm, false)
}

--- field-call-accent-eval-order-shadowed-push eval ---
// This differs because `push` is a mutating method name, even though the method
// on `sp` here isn't actually mutating.
#{
  let sp = symbol("p", ("push", sym.tilde))
  // Error: 8-15 type boolean has no method `push`
  test(sp.push(let sp = false), sym.tilde(none))
  test(sp, false)
}

--- math-field-call-accent-eval-order-shadowed-push eval ---
// Math doesn't support mutable methods and always evaluates arguments second.
#{
  let sp = symbol("p", ("push", sym.tilde))
  test($sp.push(#let sp = false;)$, $#sym.tilde(none)$)
  test(sp, false)
}

--- field-call-accent-indirect-non-mut eval ---
// Using a non-mutating method, `dict.sym.push()`, in its own argument, but
// indirectly via a mutating method, `sym-sym.pop()`.
#{
  let sp-dict = (sym: symbol("p", ("push", sym.tilde)))
  let array = ("sym", "sym")
  let result = sp-dict
    .at(array.pop())
    .push(
      sp-dict.at(array.pop()).push(none)
    )
  test(result, sym.tilde(sym.tilde(none)))
  test(array, ())
}

--- field-call-accent-assign-during-non-mut-access eval ---
// Using a non-mutating method, `dict.sym.push()`, in an assignment, but
// indirectly via a mutating method, `sym-sym.pop()`.
#{
  let sp-dict = (sym: symbol("p", ("push", sym.tilde)))
  let array = ("sym", "sym")
  sp-dict.at(array.pop()) = sp-dict.at(array.pop()).push(none)
  test(array, ())
}
//@@ file tests/suite/scripting_module.typ
// SKIP
// A file to import in import / include tests.
#import "modules/chap2.typ"

#let a
#let b = 1
#let c = 2
#let d = 3
#let value = [hi]
#let item(a, b) = a + b
#let push(a) = a + 1
#let fn = rect.with(fill: conifer, inset: 5pt)

Some _includable_ text.
//@@ file tests/suite/scripting_modules_chap1.typ
// SKIP
#let name = "Klaus"
#let chap2-path = path("./chap2.typ")
#let chap2-str = "./chap2.typ"
#let module-path = path("../module.typ")

== Chapter 1
#name stood in a field of wheat.
//@@ file tests/suite/scripting_modules_chap2.typ
// SKIP
#import "chap1.typ"
#let name = "Peter"
#let chap1-path = path("../modules/chap1.typ")
#let module-path = path("../module.typ")

== Chapter 2
#name had not yet conceptualized their consequences.
//@@ file tests/suite/scripting_modules_cycle1.typ
// SKIP
#import "cycle2.typ": *
#let inaccessible = "wow"

This is the first element of an import cycle.
//@@ file tests/suite/scripting_modules_cycle2.typ
// SKIP
#import "cycle1.typ": *
#let val = "much cycle"

This is the second element of an import cycle.
//@@ file tests/suite/scripting_ops.typ
// Test binary expressions.

--- ops-add-content paged ---
// Test adding content.
#([*Hello* ] + [world!])

--- ops-unary-basic eval ---
// Test math operators.

// Test plus and minus.
#for v in (1, 3.14, decimal("12.43"), 12pt, 45deg, 90%, 13% + 10pt, 6.3fr) {
  // Test plus.
  test(+v, v)

  // Test minus.
  test(-v, -1 * v)
  test(--v, v)

  // Test combination.
  test(-++ --v, -v)
}

#test(-(4 + 2), 6-12)

// Addition.
#test(2 + 4, 6)
#test("a" + "b", "ab")
#test("a" + if false { "b" }, "a")
#test("a" + if true { "b" }, "ab")
#test(13 * "a" + "bbbbbb", "aaaaaaaaaaaaabbbbbb")
#test((1, 2) + (3, 4), (1, 2, 3, 4))
#test((a: 1) + (b: 2, c: 3), (a: 1, b: 2, c: 3))

--- ops-add-too-large eval ---
// Error: 3-14 value is too large
#(int.max + 1)

--- ops-binary-basic eval ---
// Subtraction.
#test(1-4, 3*-1)
#test(4cm - 2cm, 2cm)
#test(1e+2-1e-2, 99.99)

// Multiplication.
#test(2 * 4, 8)

// Division.
#test(12pt/.4, 30pt)
#test(7 / 2, 3.5)

// Combination.
#test(3-4 * 5 < -10, true)
#test({ let x; x = 1 + 4*5 >= 21 and { x = "a"; x + "b" == "ab" }; x }, true)

// With block.
#test(if true {
  1
} + 2, 3)

// Mathematical identities.
#let nums = (
  1, 3.14,
  decimal("12.45"),
  12pt, 3em, 12pt + 3em,
  45deg,
  90%,
  13% + 10pt, 5% + 1em + 3pt,
  2.3fr,
)

#for v in nums {
  // Test plus and minus.
  test(v + v - v, v)
  test(v - v - v, -v)

  // Test plus/minus and multiplication.
  test(v - v, 0 * v)
  test(v + v, 2 * v)

  // Integer or decimal addition does not give a float.
  if type(v) not in (int, decimal) {
    test(v + v, 2.0 * v)
  }

  if type(v) not in (relative, decimal) and ("pt" not in repr(v) or "em" not in repr(v)) {
    test(v / v, 1.0)
  }
}

// Make sure length, ratio and relative length
// - can all be added to / subtracted from each other,
// - multiplied with integers and floats,
// - divided by integers and floats.
#let dims = (10pt, 1em, 10pt + 1em, 30%, 50% + 3cm, 40% + 2em + 1cm)
#for a in dims {
  for b in dims {
    test(type(a + b), type(a - b))
  }

  for b in (7, 3.14) {
    test(type(a * b), type(a))
    test(type(b * a), type(a))
    test(type(a / b), type(a))
  }
}

// Test division of different numeric types with zero components.
#for a in (0pt, 0em, 0%) {
  for b in (10pt, 10em, 10%) {
    test((2 * b) / b, 2)
    test((a + b * 2) / b, 2)
    test(b / (b * 2 + a), 0.5)
  }
}

--- ops-binary-decimal eval ---
// Addition.
#test(decimal("40.1") + decimal("13.2"), decimal("53.3"))
#test(decimal("12.34330") + decimal("45.96670"), decimal("58.31000"))
#test(decimal("451113.111111111111111111111") + decimal("23.222222222222222222324"), decimal("451136.333333333333333333435"))

// Subtraction.
#test(decimal("40.1") - decimal("13.2"), decimal("26.9"))
#test(decimal("12.34330") - decimal("45.96670"), decimal("-33.62340"))
#test(decimal("1234.111111111111111111111") - decimal("0.222222222222222222324"), decimal("1233.888888888888888888787"))

// Multiplication.
#test(decimal("40.5") * decimal("9.5"), decimal("384.75"))
#test(decimal("-0.1234567890123456789012345678") * decimal("-2.0"), decimal("0.2469135780246913578024691356"))

// Division.
#test(decimal("1.0") / decimal("7.0"), decimal("0.1428571428571428571428571429"))
#test(decimal("9999991.6666") / decimal("3.0"), decimal("3333330.5555333333333333333333"))
#test(decimal("3253452.4034029359598214312040") / decimal("-49293591.4039493929532"), decimal("-0.0660015290170614346071165643"))

--- ops-binary-decimal-int eval ---
// Operations between decimal and integer.
#test(decimal("2359.123456789123456789001234") + 2, decimal("2361.123456789123456789001234"))
#test(decimal("2359.123456789123456789001234") - 2, decimal("2357.123456789123456789001234"))
#test(decimal("2359.123456789123456789001234") * 2, decimal("4718.246913578246913578002468"))
#test(decimal("2359.123456789123456789001234") / 2, decimal("1179.561728394561728394500617"))

--- ops-binary-decimal-multiplication-division-imprecision eval ---
// Test digit truncation by multiplication and division.
#test(decimal("0.7777777777777777777777777777") / 1000, decimal("0.0007777777777777777777777778"))
#test(decimal("0.7777777777777777777777777777") * decimal("0.001"), decimal("0.0007777777777777777777777778"))

--- ops-add-too-large-decimal eval ---
// Error: 3-47 value is too large
#(decimal("79228162514264337593543950335") + 1)

--- ops-subtract-too-large-decimal eval ---
// Error: 3-48 value is too large
#(decimal("-79228162514264337593543950335") - 1)

--- ops-multiply-inf-with-length paged ---
// Test that multiplying infinite numbers by certain units does not crash.
#(float("inf") * 1pt)
#(float("inf") * 1em)
#(float("inf") * (1pt + 1em))

--- ops-attempt-nan-length eval ---
// Test that trying to produce a NaN scalar (such as in lengths) does not crash.
#let infpt = float("inf") * 1pt
#test(infpt - infpt, 0pt)
#test(infpt + (-infpt), 0pt)
// TODO: this result is surprising
#test(infpt / float("inf"), 0pt)

--- ops-unary-bool eval ---
// Test boolean operators.

// Test not.
#test(not true, false)
#test(not false, true)

// And.
#test(false and false, false)
#test(false and true, false)
#test(true and false, false)
#test(true and true, true)

// Or.
#test(false or false, false)
#test(false or true, true)
#test(true or false, true)
#test(true or true, true)

// Short-circuiting.
#test(false and dont-care, false)
#test(true or dont-care, true)

--- ops-equality eval ---
// Test equality operators.

// Most things compare by value.
#test(1 == "hi", false)
#test(1 == 1.0, true)
#test(30% == 30% + 0cm, true)
#test(1in == 0% + 72pt, true)
#test(30% == 30% + 1cm, false)
#test("ab" == "a" + "b", true)
#test(() == (1,), false)
#test((1, 2, 3) == (1, 2.0) + (3,), true)
#test((:) == (a: 1), false)
#test((a: 2 - 1.0, b: 2) == (b: 2, a: 1), true)
#test("a" != "a", false)
#test(decimal("1.234") == decimal("1.23400000000"), true)
#test(235 == decimal("235.0"), true)

// Functions compare by identity.
#test(test == test, true)
#test((() => {}) == (() => {}), false)

// Content compares field by field.
#let t = [a]
#test(t == t, true)
#test([] == [], true)
#test([a] == [a], true)
#test(grid[a] == grid[a], true)
#test(grid[a] == grid[b], false)

--- ops-compare eval ---
// Test comparison operators.

#test(13 * 3 < 14 * 4, true)
#test(5 < 10, true)
#test(5 > 5, false)
#test(5 <= 5, true)
#test(5 <= 4, false)
#test(45deg < 1rad, true)
#test(10% < 20%, true)
#test(50% < 40% + 0pt, false)
#test(40% + 0pt < 50% + 0pt, true)
#test(1em < 2em, true)
#test((0, 1, 2, 4) < (0, 1, 2, 5), true)
#test((0, 1, 2, 4) < (0, 1, 2, 3), false)
#test((0, 1, 2, 3.3) > (0, 1, 2, 4), false)
#test((0, 1, 2) < (0, 1, 2, 3), true)
#test((0, 1, "b") > (0, 1, "a", 3), true)
#test((0, 1.1, 3) >= (0, 1.1, 3), true)
#test((0, 1, datetime(day: 1, month: 12, year: 2023)) <= (0, 1, datetime(day: 1, month: 12, year: 2023), 3), true)
#test(("a", 23, 40, "b") > ("a", 23, 40), true)
#test(() <= (), true)
#test(() >= (), true)
#test(() <= (1,), true)
#test((1,) <= (), false)
#test(decimal("123.0000000000000000000000001") > decimal("123.0"), true)
#test(decimal("123.5") < decimal("122.444"), false)
#test(decimal("459.9999999999999999999999999") < 460, true)
#test(decimal("128.50") > 460, false)

--- ops-in eval ---
// Test `in` operator.
#test("hi" in "worship", true)
#test("hi" in ("we", "hi", "bye"), true)
#test("Hey" in "abHeyCd", true)
#test("Hey" in "abheyCd", false)
#test(5 in range(10), true)
#test(12 in range(10), false)
#test("" in (), false)
#test("key" in (key: "value"), true)
#test("value" in (key: "value"), false)
#test("Hey" not in "abheyCd", true)
#test("a" not
/* fun comment? */ in "abc", false)
#test("sys" in std, true)
#test("system" in std, false)

--- ops-in-cannot-apply-to-elem eval ---
// If this will become possible, the contains function needs to be adjusted to
// consider feature gates.
// Error: 3-25 cannot apply 'in' to string and function
#("whatever" in document)

--- ops-not-trailing eval ---
// Error: 10 expected keyword `in`
#("a" not)

--- func-with eval ---
// Test `with` method.

// Apply positional arguments.
#let add(x, y) = x + y
#test(add.with(2)(3), 5)
#test(add.with(2, 3)(), 5)
#test(add.with(2).with(3)(), 5)
#test((add.with(2))(4), 6)
#test((add.with(2).with(3))(), 5)

// Make sure that named arguments are overridable.
#let inc(x, y: 1) = x + y
#test(inc(1), 2)

#let inc2 = inc.with(y: 2)
#test(inc2(2), 4)
#test(inc2(2, y: 4), 6)

// Apply arguments to an argument sink.
#let times(..sink) = {
  let res = sink.pos().product()
  if sink.named().at("negate", default: false) { res *= -1 }
  res
}
#test((times.with(2, negate: true).with(5))(), -10)
#test((times.with(2).with(5).with(negate: true))(), -10)
#test((times.with(2).with(5, negate: true))(), -10)
#test((times.with(2).with(negate: true))(5), -10)

--- ops-precedence-basic eval ---
// Multiplication binds stronger than addition.
#test(1+2*-3, -5)

// Subtraction binds stronger than comparison.
#test(3 == 5 - 2, true)

// Boolean operations bind stronger than '=='.
#test("a" == "a" and 2 < 3, true)
#test(not "b" == "b", false)

--- ops-precedence-boolean-ops eval ---
// Assignment binds stronger than boolean operations.
#let x = false
// Error: 3-8 cannot mutate a temporary value
#(not x = "a")

--- ops-precedence-unary eval ---
// Precedence doesn't matter for chained unary operators.
// Error: 3-12 cannot apply '-' to boolean
#(-not true)

--- ops-precedence-not-in eval ---
// Not in handles precedence.
#test(-1 not in (1, 2, 3), true)

--- ops-precedence-parentheses eval ---
// Parentheses override precedence.
#test((1), 1)
#test((1+2)*-3, -9)

// Error: 8-9 unclosed delimiter
#test({(1 + 1}, 2)

--- ops-associativity-left eval ---
// Math operators are left-associative.
#test(10 / 2 / 2 == (10 / 2) / 2, true)
#test(10 / 2 / 2 == 10 / (2 / 2), false)
#test(1 / 2 * 3, 1.5)

--- ops-associativity-right eval ---
// Assignment is right-associative.
#{
  let x = 1
  let y = 2
  x = y = "ok"
  test(x, none)
  test(y, "ok")
}

--- ops-unary-minus-missing-expr eval ---
// Error: 4 expected expression
#(-)

--- ops-unary-embedded-error eval ---
// Error: 2-3 unexpected minus
// Hint: 2-3 to use a unary operator here, wrap the entire expression in parentheses
#-30deg
// Error: 2-3 unexpected plus
// Hint: 2-3 to use a unary operator here, wrap the entire expression in parentheses
#+.5
// Error: 2-5 unexpected operator `not`
// Hint: 2-5 to use a unary operator here, wrap the entire expression in parentheses
#not
// Error: 23-26 unexpected operator `not`
// Hint: 23-26 to use a unary operator here, wrap the entire expression in parentheses
$harpoon(i, dotless: #not false)$

--- ops-unary-context-error eval ---
// Error: 10-11 unexpected minus
// Hint: 10-11 to use a unary operator here, wrap the entire expression in parentheses
#context -text.size
// Error: 10-13 unexpected operator `not`
// Hint: 10-13 to use a unary operator here, wrap the entire expression in parentheses
#context not par.justify

--- ops-add-missing-rhs eval ---
// Error: 10 expected expression
#test({1+}, 1)

--- ops-mul-missing-rhs eval ---
// Error: 10 expected expression
#test({2*}, 2)

--- ops-bad-token-rhs eval ---
// Error: 7-10 invalid number suffix: `p`
#(1 + 12p)
// Error: 7-8 the character `~` is not valid in code
#(1 / ~)

--- ops-bad-token-lhs eval ---
// Error: 3-4 the character `\` is not valid in code
// Error: 5-6 unexpected star
#(\ * 1)
// Error: 3-7 unclosed label
// Error: 8-11 unexpected operator `and`
#(<lbl and 1)

--- ops-unary-plus-on-content eval ---
// Error: 3-13 cannot apply unary '+' to content
#(+([] + []))

--- ops-unary-plus-on-string eval ---
// Error: 3-6 cannot apply '-' to string
#(-"")

--- ops-not-on-array eval ---
// Error: 3-9 cannot apply 'not' to array
#(not ())

--- ops-compare-relative-length-and-ratio eval ---
// Error: 3-19 cannot compare relative length and ratio
#(30% + 1pt <= 40%)

--- ops-compare-em-with-abs eval ---
// Error: 3-14 cannot compare 1em with 10pt
#(1em <= 10pt)

--- ops-compare-normal-float-with-nan eval ---
// Error: 3-22 cannot compare 2.2 with float.nan
#(2.2 <= float("nan"))

--- ops-compare-int-and-str eval ---
// Error: 3-26 cannot compare integer and string
#((0, 1, 3) > (0, 1, "a"))

--- ops-compare-array-nested-failure eval ---
// Error: 3-42 cannot compare 3.5 with float.nan
#((0, "a", 3.5) <= (0, "a", float("nan")))

--- ops-divide-by-zero-float eval ---
// Error: 3-12 cannot divide by zero
#(1.2 / 0.0)

--- ops-divide-by-zero-int eval ---
// Error: 3-8 cannot divide by zero
#(1 / 0)

--- ops-divide-by-zero-angle eval ---
// Error: 3-15 cannot divide by zero
#(15deg / 0deg)

--- ops-binary-arithmetic-error-message eval ---
// Special messages for +, -, * and /.
// Error: 3-10 cannot add integer and string
#(1 + "2", 40% - 1)

--- add-assign-int-and-str eval ---
// Error: 15-23 cannot add integer and string
#{ let x = 1; x += "2" }

--- ops-divide-ratio-by-length eval ---
// Error: 4-13 cannot divide ratio by length
#( 10% / 5pt )

--- ops-divide-em-by-abs eval ---
// Error: 3-12 cannot divide these two lengths
#(1em / 5pt)

--- ops-divide-relative-length-by-ratio eval ---
// Error: 3-19 cannot divide relative length by ratio
#((10% + 1pt) / 5%)

--- ops-divide-relative-lengths eval ---
// Error: 3-28 cannot divide these two relative lengths
#((10% + 1pt) / (20% + 1pt))

--- ops-subtract-int-from-ratio eval ---
// Error: 13-20 cannot subtract integer from ratio
#((1234567, 40% - 1))

--- ops-multiply-int-with-bool eval ---
// Error: 3-11 cannot multiply integer with boolean
#(2 * true)

--- ops-divide-int-by-length eval ---
// Error: 3-11 cannot divide integer by length
#(3 / 12pt)

--- multiply-negative-int-with-str eval ---
// Error: 3-10 number must be at least zero
#(-1 * "")

--- ops-assign eval ---
// Test assignment operators.

#let x = 0
#(x = 10)       #test(x, 10)
#(x -= 5)       #test(x, 5)
#(x += 1)       #test(x, 6)
#(x *= x)       #test(x, 36)
#(x /= 2.0)     #test(x, 18.0)
#(x = "some")   #test(x, "some")
#(x += "thing") #test(x, "something")

--- ops-assign-unknown-var-lhs eval ---
#{
  // Error: 3-6 unknown variable `a-1`
  // Hint: 3-6 if you meant to use subtraction, try adding spaces around the minus sign: `a - 1`
  a-1 = 2
}

--- ops-assign-unknown-var-rhs eval ---
#{
  let a = 2
  a = 1-a
  a = a -1

  // Error: 7-10 unknown variable `a-1`
  // Hint: 7-10 if you meant to use subtraction, try adding spaces around the minus sign: `a - 1`
  a = a-1
}

--- ops-assign-unknown-parenthesized-variable eval ---
// Error: 4-5 unknown variable `x`
#((x) = "")

--- ops-assign-destructuring-unknown-variable eval ---
// Error: 4-5 unknown variable `x`
#((x,) = (1,))

--- ops-assign-to-temporary eval ---
// Error: 3-8 cannot mutate a temporary value
#(1 + 2 += 3)

--- ops-assign-to-temporary-method eval ---
#let numbers = (3, 2, 1)
// Error: 3-19 cannot mutate a temporary value
#(numbers.sorted() = (1, 2, 3))

--- ops-assign-to-invalid-unary-op eval ---
#let x = "Hey"
// Error: 3-8 cannot apply 'not' to string
#(not x = "a")

--- ops-assign-to-invalid-binary-op eval ---
// Error: 7-8 unknown variable `x`
#(1 + x += 3)

--- ops-assign-unknown-variable eval ---
// Error: 3-4 unknown variable `z`
#(z = 1)

--- ops-assign-to-std-constant eval ---
// Error: 3-7 cannot mutate a constant: rect
#(rect = "hi")

--- ops-assign-to-shadowed-std-constant eval ---
// Works if we define rect beforehand
// (since then it doesn't resolve to the standard library version anymore).
#let rect = ""
#(rect = "hi")

--- ops-assign-shadow-eval-order eval ---
// Test shadowing a variable while assigning to it and calling a method on it.
#{
  let var = "a"
  var += var.at(0, default: let var = "b")
  test(var, "ba")
}
//@@ file tests/suite/scripting_params.typ
--- param-underscore-missing-argument eval ---
// Error: 17-20 missing argument: pattern parameter
#let f(a: 10) = a() + 1
#f(a: _ => 5)

--- params-sink-named eval ---
// ... but this was.
#let f(..x) = {}
#f(arg: 1)

--- params-sink-unnamed eval ---
// unnamed spread
#let f(.., a) = a
#test(f(1, 2, 3), 3)

// This wasn't allowed before the bug fix ...
#let f(..) = 2
#test(f(arg: 1), 2)

--- params-sink-bool-invalid eval ---
// Error: 10-14 expected pattern, found boolean
#let f(..true) = none

--- params-sink-multiple-invalid eval ---
// Error: 13-16 only one argument sink is allowed
#let f(..a, ..b) = none

--- params-sink-at-start eval ---
// Spread at beginning.
#{
  let f(..a, b) = (a, b)
  test(repr(f(1)), "(arguments(), 1)")
  test(repr(f(1, 2, 3)), "(arguments(1, 2), 3)")
  test(repr(f(1, 2, 3, 4, 5)), "(arguments(1, 2, 3, 4), 5)")
}

--- params-sink-in-middle eval ---
// Spread in the middle.
#{
  let f(a, ..b, c) = (a, b, c)
  test(repr(f(1, 2)), "(1, arguments(), 2)")
  test(repr(f(1, 2, 3, 4, 5)), "(1, arguments(2, 3, 4), 5)")
}

--- params-sink-unnamed-empty eval ---
// Unnamed sink should just ignore any extra arguments.
#{
  let f(a, b: 5, ..) = (a, b)
  test(f(4), (4, 5))
  test(f(10, b: 11), (10, 11))
  test(f(13, 20, b: 12), (13, 12))
  test(f(15, b: 16, c: 13), (15, 16))
}

--- params-sink-missing-arguments eval ---
#{
  let f(..a, b, c, d) = none

  // Error: 3-10 missing argument: d
  f(1, 2)
}

--- params-missing-arguments-with eval ---
#{
  let f(a, b, c) = none
  let f = f.with(1)
  // Error: 3-7 missing argument: c
  f(2)
}

--- params-missing-arguments-with-callback eval trace ---
#{
  // Error: 8-17 missing argument: c
  let f(a, b, c) = none
  let f = f.with(1)
  // Trace: 3-24 (2) while calling `position`
  // Trace: 22-23 (1) while calling `f`
  (1, 2, 3).position(f)
}

--- issue-1029-parameter-destructuring eval ---
// Test that underscore works in parameter patterns.
#test((1, 2, 3).zip((1, 2, 3)).map(((_, x)) => x), (1, 2, 3))

--- issue-1351-parameter-dictionary eval ---
// Error: 17-22 expected pattern, found string
#let foo((test: "bar")) = {}
//@@ file tests/suite/scripting_recursion.typ
// Test recursive function calls.

--- recursion-named eval ---
// Test with named function.
#let fib(n) = {
  if n <= 2 {
    1
  } else {
    fib(n - 1) + fib(n - 2)
  }
}

#test(fib(10), 55)

--- recursion-unnamed-invalid eval ---
// Test with unnamed function.
// Error: 17-18 unknown variable `f`
#let f = (n) => f(n - 1)
#f(10)

--- recursion-named-returns-itself eval ---
// Test capturing with named function.
#let f = 10
#let f() = f
#test(type(f()), function)

--- recursion-unnamed-does-not-return-itself eval ---
// Test capturing with unnamed function.
#let f = 10
#let f = () => f
#test(type(f()), int)

--- recursion-shadowing eval ---
// Test redefinition.
#let f(x) = "hello"
#let f(x) = if x != none { f(none) } else { "world" }
#test(f(1), "world")

--- recursion-maximum-depth eval ---
// Error: 15-21 maximum function call depth exceeded
#let rec(n) = rec(n) + 1
#rec(1)

--- recursion-via-include-in-layout paged ---
// Test cyclic imports during layout.
// Error: 2-38 maximum show rule depth exceeded
// Hint: 2-38 maybe a show rule matches its own output
// Hint: 2-38 maybe there are too deeply nested elements
#layout(_ => include "recursion.typ")

--- recursion-show-math paged ---
// Test recursive show rules.
// Error: 22-25 maximum show rule depth exceeded
// Hint: 22-25 maybe a show rule matches its own output
// Hint: 22-25 maybe there are too deeply nested elements
#show math.equation: $x$
$ x $

--- recursion-show-math-realize paged ---
// Error: 22-33 maximum show rule depth exceeded
// Hint: 22-33 maybe a show rule matches its own output
// Hint: 22-33 maybe there are too deeply nested elements
#show heading: it => heading[it]
$ #heading[hi] $
//@@ file tests/suite/scripting_return.typ
// Test return out of functions.

--- return-with-value eval ---
// Test return with value.
#let f(x) = {
  return x + 1
}

#test(f(1), 2)

--- return-join eval ---
// Test return with joining.

#let f(x) = {
  "a"
  if x == 0 {
    return "b"
  } else if x == 1 {
    "c"
  } else {
    "d"
    return
    "e"
  }
}

#test(f(0), "b")
#test(f(1), "ac")
#test(f(2), "ad")

--- return-in-nested-content-block paged ---
// Test return with joining and content.

#let f(text, caption: none) = {
  text
  if caption == none [\.#return]
  [, ]
  emph(caption)
  [\.]
}

#f(caption: [with caption])[My figure]

#f[My other figure]

--- return-outside-of-function eval ---
// Test return outside of function.

#for x in range(5) {
  // Error: 3-9 cannot return outside of function
  return
}

--- return-in-first-arg eval ---
// Test that the expression is evaluated to the end.
#let sum(..args) = {
  let s = 0
  for v in args.pos() {
    s += v
  }
  s
}

#let f() = {
  sum(..return, 1, 2, 3)
  "nope"
}

#test(f(), 6)

--- return-in-content-block eval ---
// Test value return from content.
#let x = 3
#let f() = [
  Hello 😀
  #return "nope"
  World
]

#test(f(), "nope")

--- return-semicolon-or-linebreak eval ---
// Test rejection of extra value
#let f() = [
  // Error: 16-16 expected semicolon or line break
  #return a + b Hello World
]

--- return-discard-content eval ---
// Test that discarding joined content is a warning.
#let f() = {
  [Hello, World!]
  // Warning: 3-16 this return unconditionally discards the content before it
  // Hint: 3-16 try omitting the `return` to automatically join all values
  return "nope"
}

#test(f(), "nope")

--- return-discard-content-nested eval ---
#let f() = {
  [Hello, World!]
  {
    // Warning: 5-18 this return unconditionally discards the content before it
    // Hint: 5-18 try omitting the `return` to automatically join all values
    return "nope"
  }
}

#test(f(), "nope")

--- return-discard-state eval ---
// Test that discarding a joined content with state is special warning

#let f() = {
  state("hello").update("world")

  // Warning: 3-19 this return unconditionally discards the content before it
  // Hint: 3-19 try omitting the `return` to automatically join all values
  // Hint: 3-19 state/counter updates are content that must end up in the document to have an effect
  return [ Hello ]
}

#test(f(), [ Hello ])

--- return-discard-loop eval ---
// Test that return from within a control flow construct is not a warning.
#let f1() = {
  state("hello").update("world")
  for x in range(3) {
    return "nope1"
  }
}

#let f2() = {
  state("hello").update("world")
  let i = 0
  while i < 10 {
    return "nope2"
  }
}

#test(f1(), "nope1")
#test(f2(), "nope2")

--- return-no-discard eval ---
// Test that returning the joined content is not a warning.
#let f() = {
  state("hello").update("world")
  return
}

#test(f(), state("hello").update("world"))

--- return-discard-not-content eval ---
// Test that non-content joined value is not a warning.
#let f() = {
  (33,)
  return (66,)
}

#test(f(), (66, ))

--- return-discard-markup eval ---
// Test that discarding markup is not a warning.
#let f() = [
  hello
  #return [nope]
]

#test(f(), [nope])
//@@ file tests/suite/scripting_while.typ
// Test while expressions.

--- while-loop-basic paged ---
// Should output `2 4 6 8 10`.
#let i = 0
#while i < 10 [
  #(i += 2)
  #i
]

// Should output `Hi`.
#let iter = true
#while iter {
  iter = false
  "Hi."
}

#while false {
  dont-care
}

--- while-loop-expr eval ---
// Value of while loops.

#test(while false {}, none)

#let i = 0
#test(type(while i < 1 [#(i += 1)]), content)

--- while-loop-condition-content-invalid eval ---
// Condition must be boolean.
// Error: 8-14 expected boolean, found content
#while [nope] [nope]

--- while-loop-condition-always-true eval ---
// Error: 8-25 condition is always true
#while 2 < "hello".len() {}

--- while-loop-limit eval ---
#let i = 1
// Error: 2-24 loop seems to be infinite
#while i > 0 { i += 1 }

--- while-loop-incomplete eval ---
// Error: 7 expected expression
#while

// Error: 8 expected expression
#{while}

// Error: 9 expected block
#while x

// Error: 7 expected expression
#while
x {}

// Error: 9 expected block
#while x something
//@@ file tests/suite/styling_fold.typ
--- fold-vec-order-text-features paged ---
// Test fold order of vectors.
#set text(features: (liga: 1))
#set text(features: (liga: 0))
fi

--- fold-vec-order-text-decos paged ---
#underline(stroke: aqua + 4pt)[
  #underline[Hello]
]

--- fold-vec-order-meta paged ---
#let c = counter("mycounter")
#c.update(1)

#context [
  #c.update(2)
  #c.get() \
  Second: #context c.get()
]
//@@ file tests/suite/styling_set.typ
// General tests for set.

--- set-instantiation-site paged ---
// Test that text is affected by instantiation-site bold.
#let x = [World]
Hello *#x*

--- set-instantiation-site-markup paged ---
// Test that lists are affected by correct indents.
#let fruit = [
  - Apple
  - Orange
  #list(body-indent: 20pt)[Pear]
]

- Fruit
#[#set list(indent: 10pt)
 #fruit]
- No more fruit

--- set-text-override paged ---
// Test that block spacing and text style are respected from
// the outside, but the more specific fill is respected.
#set par(spacing: 4pt)
#set text(style: "italic", fill: eastern)
#let x = [And the forest #parbreak() lay silent!]
#text(fill: forest, x)

--- set-scoped-in-code-block paged ---
// Test that scoping works as expected.
#{
  if true {
    set text(blue)
    [Blue ]
  }
  [Not blue]
}

--- closure-path-resolve-in-layout-phase paged ---
// Test relative path resolving in layout phase.
#let choice = ("monkey.svg", "rhino.png", "tiger.jpg")
#set enum(numbering: n => {
  let path = "/assets/images/" + choice.at(n - 1)
  move(dy: -0.15em, image(path, width: 1em, height: 1em))
})

+ Monkey
+ Rhino
+ Tiger

--- set-if paged ---
// Test conditional set.
#show ref: it => {
  set text(red) if it.target == <unknown>
  "@" + str(it.target)
}

@hello from the @unknown

--- set-if-bad-type eval ---
// Error: 19-24 expected boolean, found integer
#set text(red) if 1 + 2

--- set-shadowed-builtin eval ---
#let text = "foo"
// Error: 6-10 expected function, found string
// Hint: 6-10 use `std.text` to access the shadowed standard library function
#set text(fill: red)

--- set-shadowed-builtin-with-std paged ---
#let text = "bar"
#set std.text(fill: red)
#text

--- set-in-expr eval ---
// Error: 12-26 set is only allowed directly in code and content blocks
#{ let x = set text(blue) }

--- set-bad-trivia eval ---
// Error cases parsing set rules with trivia between the function and args.
// Error: 10 expected argument list
#set page
         (numbering: "1")
// Error: 10 expected argument list
// Hint: 10 there may not be any spaces before the argument list
#set page (numbering: "2")
// Error: 10 expected argument list
// Hint: 10 there may not be any spaces before the argument list
#set page/**/(numbering: "3")

// This is fine though
#set/**/page(numbering: "4")

--- set-vs-construct-1 paged ---
// Ensure that constructor styles aren't passed down the tree.
// The inner list should have no extra indent.
#set par(leading: 2pt)
#list(body-indent: 20pt, [First], list[A][B])

--- set-vs-construct-2 paged ---
// Ensure that constructor styles win, but not over outer styles.
// The outer paragraph should be right-aligned,
// but the B should be center-aligned.
#set list(marker: [>])
#list(marker: [--])[
  #rect(width: 2cm, fill: conifer, inset: 4pt, list[A])
]

--- set-vs-construct-3 paged ---
// The inner rectangle should also be yellow here.
// (and therefore invisible)
#[#set rect(fill: yellow);#text(1em, rect(inset: 5pt, rect()))]

--- set-vs-construct-4 paged ---
// The inner rectangle should not be yellow here.
A #box(rect(fill: yellow, inset: 5pt, rect())) B

--- show-set-vs-construct paged ---
// The constructor property should still work
// when there are recursive show rules.
#show enum: set text(blue)
#enum(numbering: "(a)", [A], enum[B])
//@@ file tests/suite/styling_show-set.typ
// Test show-set rules.

--- show-set-override paged ---
// Test overriding show-set rules.
#show strong: set text(red)
Hello *World*

#show strong: set text(blue)
Hello *World*

--- show-set-on-same-element paged ---
// Test show-set rule on the same element.
#set figure(supplement: [Default])
#show figure.where(kind: table): set figure(supplement: [Tableau])
#figure(
  table(columns: 2)[A][B][C][D],
  caption: [Four letters],
)

--- show-set-same-element-and-order paged ---
// Test both things at once.
#show heading: set text(red)
= Level 1
== Level 2

#show heading.where(level: 1): set text(blue)
#show heading.where(level: 1): set text(green)
#show heading.where(level: 1): set heading(numbering: "(I)")
= Level 1
== Level 2

--- show-set-same-element-matched-field paged ---
// Test setting the thing we just matched on.
// This is quite cursed, but it works.
#set heading(numbering: "(I)")
#show heading.where(numbering: "(I)"): set heading(numbering: "1.")
= Heading

--- show-set-same-element-synthesized-matched-field paged ---
// Same thing, but even more cursed, because `kind` is synthesized.
#show figure.where(kind: table): set figure(kind: raw)
#figure(table[A], caption: [Code])

--- show-set-same-element-matching-interaction paged ---
// Test that show-set rules on the same element don't affect each other. This
// could be implemented, but isn't as of yet.
#show heading.where(level: 1): set heading(numbering: "(I)")
#show heading.where(numbering: "(I)"): set text(red)
= Heading

--- show-set-on-layoutable-element paged ---
// Test show-set rules on layoutable element to ensure it is realized
// even though it implements `LayoutMultiple`.
#show table: set text(red)
#pad(table(columns: 4)[A][B][C][D])

--- show-function-order-with-set paged ---
// These are both red because in the expanded form, `set text(red)` ends up
// closer to the content than `set text(blue)`.
#show strong: it => { set text(red); it }
Hello *World*

#show strong: it => { set text(blue); it }
Hello *World*

--- show-function-set-on-it paged ---
// This doesn't have an effect. An element is materialized before any show
// rules run.
#show heading: it => { set heading(numbering: "(I)"); it }
= Heading
//@@ file tests/suite/styling_show-text.typ
// Test text replacement show rules.

--- show-text-basic paged ---
// Test classic example.
#set text(font: "Roboto")
#show "Der Spiegel": smallcaps
Die Zeitung Der Spiegel existiert.

--- show-text-regex paged ---
// Another classic example.
#show "TeX": [T#h(-0.145em)#box(move(dy: 0.233em)[E])#h(-0.135em)X]
#show regex("(Lua)?(La)?TeX"): name => box(text(font: "New Computer Modern")[#name])

TeX, LaTeX, LuaTeX and LuaLaTeX!

--- show-text-cyclic paged ---
// Test direct cycle.
#show "Hello": text(red)[Hello]
Hello World!

--- show-text-cyclic-raw paged ---
// Test replacing text with raw text.
#show "rax": `rax`
The register rax.

--- show-text-indirectly-cyclic paged ---
// Test indirect cycle.
#show "Good": [Typst!]
#show "Typst": [Fun!]
#show "Fun": [Good!]

#set text(ligatures: false)
Good \
Fun \
Typst \

--- show-text-exactly-once paged ---
// Test that replacements happen exactly once.
#show "A": [BB]
#show "B": [CC]
AA (8)

--- show-text-regex-word-boundary paged ---
// Test caseless match and word boundaries.
#show regex("(?i)\\bworld\\b"): [🌍]

Treeworld, the World of worlds, is a world.

--- show-text-empty eval ---
// Test there is no crashing on empty strings
// Error: 7-9 text selector is empty
#show "": []

--- show-text-regex-empty eval ---
// Error: 7-16 regex selector is empty
#show regex(""): [AA]

--- show-text-regex-matches-empty eval ---
// Error: 7-42 regex matches empty text
#show regex("(VAR_GLOBAL|END_VAR||BOOL)") : []

--- show-text-regex-character-class paged ---
// This is a fun one.
#set par(justify: true)
#show regex("\\S"): letter => box(stroke: 1pt, inset: 2pt, upper(letter))
#lorem(5)

--- show-text-regex-case-insensitive paged ---
// See also: https://github.com/mTvare6/hello-world.rs
#show regex("(?i)rust"): it => [#it (🚀)]
Rust is memory-safe and blazingly fast. Let's rewrite everything in rust.

--- show-text-get-text-on-it paged ---
// Test accessing the string itself.
#show "hello": it => it.text.split("").map(upper).join("|")
Oh, hello there!

--- show-text-in-other-show paged ---
// Replace worlds but only in lists.
#show list: it => [
  #show "World": [🌎]
  #it
]

World
- World

--- show-text-path-resolving paged ---
// Test absolute path in layout phase.

#show "GRAPH": image("/assets/images/graph.png")

The GRAPH has nodes.

--- show-set-text-order-adjacent-1 paged ---
#show "He": set text(red)
#show "ya": set text(blue)
Heya

--- show-set-text-order-contained-1 paged ---
#show "Heya": set text(red)
#show   "ya": set text(blue)
Heya

--- show-set-text-order-contained-3 paged ---
#show "He": set text(red)
#show "Heya": set text(blue)
Heya

--- show-set-text-order-overlapping-1 paged ---
#show "Heya": set text(red)
#show   "yaho": set text(blue)
Heyaho

--- show-set-text-order-adjacent-2 paged ---
#show "He": set text(red)
#show "ya": set text(weight: "bold")
Heya

--- show-set-text-order-contained-2 paged ---
#show "Heya": set text(red)
#show   "ya": set text(weight: "bold")
Heya

--- show-set-text-order-contained-4 paged ---
#show "He": set text(red)
#show "Heya": set text(weight: "bold")
Heya

--- show-set-text-order-overlapping-2 paged ---
#show "Heya": set text(red)
#show   "yaho": set text(weight: "bold")
Heyaho

--- show-text-smartquote paged ---
#show "up,\" she": set text(red)
"What's up," she asked.

--- show-text-apostrophe paged ---
#show regex("Who's|We've"): highlight
Who's got it? \
We've got it.

--- show-text-citation paged ---
#show "hey": [@arrgh]
@netwok hey

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- show-text-list paged ---
#show "hi": [- B]
- A
hi
- C

--- show-text-citation-smartquote paged ---
#show "hey \"": [@arrgh]
#show "dis": [@distress]
@netwok hey " dis

#show bibliography: none
#bibliography("/assets/bib/works.bib", style: "american-physics-society")

--- show-text-in-citation paged ---
#show "A": "B"
#show "[": "("
#show "]": ")"
#show "[2]": set text(red)

@netwok A \
@arrgh B

#show bibliography: none
#bibliography("/assets/bib/works.bib")

--- show-text-linebreak paged ---
#show "lo\nwo": set text(red)
Hello #[ ] \
#[ ] #[ ] world!

--- show-text-line-wrapping paged ---
#show "start end": "word"
start
end

--- show-text-after-normal-show paged ---
#show rect: "world"
#show "lo wo": set text(red)
hello #rect()

--- show-text-space-collapsing paged ---
#show "i ther": set text(red)
hi#[ ]#[ ]the#"re"

--- show-text-style-boundary paged ---
#show "What's up": set text(blue)
#show "your party": underline
What's #[ ] up at #"your" #text(red)[party?]

--- show-text-within-par paged ---
#show "Pythagoras'": highlight
$a^2 + b^2 = c^2$ is Pythagoras' theorem.

--- show-text-outer-space paged ---
// Spaces must be interior to strong textual elements for matching to work.
// For outer spaces, it is hard to say whether they would collapse.
#show "a\n": set text(blue)
#show "b\n ": set text(blue)
#show " c ": set text(blue)
a \ #h(0pt, weak: true)
b \ #h(0pt, weak: true)
$x$ c $y$

--- show-text-styled-space paged ---
// Differently styled spaces between text are not matched by regex rules.
// This is solely due to grouping rules, not space collapsing.
#show " ": "B"
#show "X": "B"
A C \
A#text(red)[ ]C \
A#text(red)[X]C

--- show-text-isolated-space paged ---
#show " ": it => {
  test(it.func(), text)
  test(it.text, " ")
  [-]
}
// We split up the text run into three separate elements to see what kind of
// element we get in the match (space vs text). We want text so that a `.text`
// field is available.
A#[ ]B

--- show-text-where-text-is-space paged ---
// Spaces in `text` match even if differently styled, unlike the previous test.
#show text.where(text: " "): [B]
A#text(" ")C \
A#text([ ])C \ // Space elements don't magically become `text`
A#text(" ", red)C

--- issue-5014-show-text-tags paged ---
#{
  let c = counter("c")
  show "b": context c.get().first()
  [a]
  c.step()
  [bc]
}

--- issue-7991-show-text-symbol-grapheme-sliced paged ---
#show "*": set text(red)
$ast.basic$

--- issue-8058-show-text-grapheme-sliced paged ---
// The issue is not yet fixed, so this test demonstrates the current undesirable
// behavior.

#set text(font: "New Computer Modern Math")

#let emptyset = sym.emptyset
#let narrowemptysettext = emptyset + "\u{fe00}"
#let narrowemptyset = symbol(narrowemptysettext)

// This should still be narrow, but it's currently not because the symbol is
// split up and the shaper gets both parts separately. This could probably also
// be fixed in the inline layout.
//
// See also https://github.com/typst/typst/issues/8058.
#[
  #show emptyset: it => it
  #narrowemptysettext, $narrowemptyset$
]

// Same here.
#[
  #show "\u{fe00}": it => it
  #narrowemptysettext, $narrowemptyset$
]
//@@ file tests/suite/styling_show-where.typ
--- show-where-optional-field-raw paged ---
// Test that where selectors also trigger on set rule fields.
#show raw.where(block: false): box.with(
  fill: luma(220),
  inset: (x: 3pt, y: 0pt),
  outset: (y: 3pt),
  radius: 2pt,
)

This is #raw("fn main() {}") some text.

--- show-where-optional-field-text paged ---
// Note: This show rule is horribly inefficient because it triggers for
// every individual text element. But it should still work.
#show text.where(lang: "de"): set text(red)

#set text(lang: "es")
Hola, mundo!

#set text(lang: "de")
Hallo Welt!

#set text(lang: "en")
Hello World!

--- show-where-folding-text-size paged ---
// Test that folding is taken into account.
#set text(5pt)
#set text(2em)

#[
  #show text.where(size: 2em): set text(blue)
  2em not blue
]

#[
  #show text.where(size: 10pt): set text(blue)
  10pt blue
]

--- show-where-folding-stroke paged ---
// Test again that folding is taken into account.
#set rect(width: 40pt, height: 10pt)
#set rect(stroke: blue)
#set rect(stroke: 2pt)

#{
  show rect.where(stroke: blue): "Not Triggered"
  rect()
}
#{
  show rect.where(stroke: 2pt): "Not Triggered"
  rect()
}
#{
  show rect.where(stroke: 2pt + blue): "Triggered"
  rect()
}

--- show-where-resolving-length paged ---
// Test that resolving is *not* taken into account.
#set line(start: (1em, 1em + 2pt))

#{
  show line.where(start: (1em, 1em + 2pt)): "Triggered"
  line()
}
#{
  show line.where(start: (10pt, 12pt)): "Not Triggered"
  line()
}


--- show-where-resolving-hyphenate paged ---
// Test again that resolving is *not* taken into account.
#set text(hyphenate: auto)

#[
  #show text.where(hyphenate: auto): underline
  Auto
]
#[
  #show text.where(hyphenate: true): underline
  True
]
#[
  #show text.where(hyphenate: false): underline
  False
]
//@@ file tests/suite/styling_show.typ
// Test show rules.

--- show-selector-basic paged ---
// Override lists.
#show list: it => "(" + it.children.map(v => v.body).join(", ") + ")"

- A
  - B
  - C
- D
- E

--- show-selector-replace-and-show-set paged ---
// Test full reset.
#show heading: [B]
#show heading: set text(size: 10pt, weight: 400)
A #[= Heading] C

--- show-selector-discard paged ---
// Test full removal.
#show heading: none

Where is
= There are no headings around here!
my heading?

--- show-selector-realistic paged ---
// Test integrated example.
#show heading: it => block({
  set text(10pt)
  box(move(dy: -1pt)[📖])
  h(5pt)
  if it.level == 1 {
    underline(text(1.25em, blue, it.body))
  } else {
    text(red, it.body)
  }
})

= Task 1
Some text.

== Subtask
Some more text.

= Task 2
Another text.

--- show-in-show paged ---
// Test set and show in code blocks.
#show heading: it => {
  set text(red)
  show "ding": [🛎]
  it.body
}

= Heading

--- show-nested-scopes paged ---
// Test that scoping works as expected.
#{
  let world = [ World ]
  show "W": strong
  world
  {
    set text(blue)
    show: it => {
      show "o": "Ø"
      it
    }
    world
  }
  world
}

--- show-selector-replace paged ---
#show heading: [1234]
= Heading

--- show-unknown-field paged ---
// Error: 25-29 heading does not have field "page"
#show heading: it => it.page
= Heading

--- show-text-element-discard paged empty ---
#show text: none
Hey

--- show-selector-not-an-element-function eval ---
// Error: 7-12 only element functions can be used as selectors
#show upper: it => {}

--- show-selector-shadowed-builtin eval ---
#let heading = 0

// Error: 7-14 expected symbol, string, label, function, regex, or selector, found integer
// Hint: 7-14 use `std.heading` to access the shadowed standard library function
#show heading: it => it

--- show-selector-shadowed-builtin-with-std paged ---
#let heading = "bar"
#show std.heading: it => text(fill: red, it)
= #heading

--- show-bad-replacement-type eval ---
// Error: 16-20 expected content or function, found integer
#show heading: 1234
= Heading

--- show-bad-selector-type eval ---
// Error: 7-10 expected symbol, string, label, function, regex, or selector, found color
#show red: []

--- show-selector-in-expression eval ---
// Error: 7-25 show is only allowed directly in code and content blocks
#(1 + show heading: none)

--- show-bare-basic paged ---
#set page(height: 130pt)
#set text(0.7em)

#align(center)[
  #text(1.3em)[*Essay on typography*] \
  T. Ypst
]

#show: columns.with(2)
#lines(16)

--- show-bare-content-block paged ---
// Test bare show in content block.
A #[_B #show: c => [*#c*]; C_] D

--- show-bare-vs-set-text paged ---
// Test style precedence.
#set text(fill: eastern, size: 1.5em)
#show: text.with(fill: forest)
Forest

--- show-bare-replace-with-content paged ---
#show: [Shown]
Ignored

--- show-bare-in-expression eval ---
// Error: 4-19 show is only allowed directly in code and content blocks
#((show: body => 2) * body)

--- show-bare-missing-colon-closure eval ---
// Error: 6 expected colon
#show it => {}

--- show-bare-missing-colon eval ---
// Error: 6 expected colon
#show it

--- show-recursive-identity paged ---
// Test basic identity.
#show heading: it => it
= Heading

--- show-multiple-rules paged ---
// Test more recipes down the chain.
#show list: scale.with(origin: left, x: 80%)
#show heading: []
#show enum: []
- Actual
- Tight
- List
= Nope

--- show-rule-in-function paged ---
// Test show rule in function.
#let starwars(body) = {
  show list: it => block({
    stack(dir: ltr,
      text(red, it),
      1fr,
      scale(x: -100%, text(blue, it)),
    )
  })
  body
}

- Normal list

#starwars[
  - Star
  - Wars
  - List
]

- Normal list

--- show-recursive-multiple paged ---
// Test multi-recursion with nested lists.
#set rect(inset: 3pt)
#show list: rect.with(stroke: blue)
#show list: rect.with(stroke: red)
#show list: block

- List
  - Nested
  - List
- Recursive!

--- show-selector-where paged ---
// Inline code.
#show raw.where(block: false): box.with(
  radius: 2pt,
  outset: (y: 2.5pt),
  inset: (x: 3pt, y: 0pt),
  fill: luma(230),
)

// Code blocks.
#show raw.where(block: true): block.with(
  outset: -3pt,
  inset: 11pt,
  fill: luma(230),
  stroke: (left: 1.5pt + luma(180)),
)

#set page(margin: (top: 12pt))
#set par(justify: true)

This code tests `code`
with selectors and justification.

```rs
code!("it");
```

You can use the ```rs *const T``` pointer or
the ```rs &mut T``` reference.

--- show-set-where-override paged ---
#show heading: set text(green)
#show heading.where(level: 1): set text(red)
#show heading.where(level: 2): set text(blue)
= Red
== Blue
=== Green

--- show-selector-or-elements-with-set paged ---
// Looking forward to `heading.where(level: 1 | 2)` :)
#show heading.where(level: 1).or(heading.where(level: 2)): set text(red)
= L1
== L2
=== L3
==== L4

--- show-selector-element-or-label paged ---
// Test element selector combined with label selector.
#show selector(strong).or(<special>): highlight
I am *strong*, I am _emphasized_, and I am #[special<special>].

--- show-selector-element-or-text eval ---
// Ensure that text selector cannot be nested in and/or. That's too complicated,
// at least for now.

// Error: 7-41 this selector cannot be used with show
#show heading.where(level: 1).or("more"): set text(red)

--- show-selector-within eval ---
// Error: 7-33 this selector cannot currently be used with show
// Hint: 7-33 support for this is planned for the future
#show selector(emph).within(par): set text(red)

--- show-delayed-error paged trace ---
// Error: 21-34 panicked with: hey1
#show heading: _ => panic("hey1")

// Error: 20-33 panicked with: hey2
#show strong: _ => panic("hey2")

// Trace: 1-8 (1) while showing heading element
= Hello
// Trace: 1-9 (1) while showing strong element
*strong*

--- show-error-trace paged trace ---
// Error: 16-34 panicked with: Heading
#let func(c) = panic(c.body.text)

// Trace: 2-20 (1) while calling `func`
#show heading: func

// Trace: 1-10 (2) while showing heading element
= Heading

--- issue-5690-oom-par-box paged ---
// Error: 3:6-5:1 maximum grouping depth exceeded
#show par: box

Hello

World
//@@ file tests/suite/symbols_symbol.typ
// Test symbols.

--- symbol paged ---
#emoji.face
#emoji.woman.old
#emoji.turtle

#set text(font: "New Computer Modern Math")
#sym.arrow
#sym.arrow.l
#sym.arrow.r.squiggly
#sym.arrow.tr.hook

#sym.arrow.r;this and this#sym.arrow.l;

--- symbol-constructor paged ---
#let envelope = symbol(
  "🖂",
  ("stamped", "🖃"),
  ("stamped.pen", "🖆"),
  ("lightning", "🖄"),
  ("fly", "🖅"),
)
#let one = symbol(
  "1",
  ("emoji", "1️")
)

#envelope
#envelope.stamped
#envelope.pen
#envelope.stamped.pen
#envelope.lightning
#envelope.fly
#one
#one.emoji

--- symbol-constructor-empty eval ---
// Error: 2-10 expected at least one variant
#symbol()

--- symbol-constructor-invalid-modifier eval ---
#symbol(
// Error: 3-24 invalid symbol modifier: " id!"
  ("invalid. id!", "x")
)

--- symbol-constructor-duplicate-modifier eval ---
#symbol(
  // Error: 3-31 duplicate modifier within variant: "duplicate"
  // Hint: 3-31 modifiers are not ordered, so each one may appear only once
  ("duplicate.duplicate", "x"),
)

--- symbol-constructor-duplicate-default-variant eval ---
#symbol(
  "x",
  // Error: 3-6 duplicate default variant
  "y",
)

--- symbol-constructor-duplicate-empty-variant eval ---
#symbol(
  ("", "x"),
  // Error: 3-12 duplicate default variant
  ("", "y"),
)

--- symbol-constructor-default-and-empty-variants eval ---
#symbol(
  "x",
  // Error: 3-12 duplicate default variant
  ("", "y"),
)

--- symbol-constructor-duplicate-variant eval ---
#symbol(
  ("duplicate.variant", "x"),
  // Error: 3-29 duplicate variant: "duplicate.variant"
  ("duplicate.variant", "y"),
)

--- symbol-constructor-duplicate-variant-different-order eval ---
#symbol(
  ("duplicate.variant", "x"),
  // Error: 3-29 duplicate variant: "variant.duplicate"
  // Hint: 3-29 variants with the same modifiers are identical, regardless of their order
  ("variant.duplicate", "y"),
)

--- symbol-constructor-empty-variant-value eval ---
#symbol(
  // Error: 3-5 invalid variant value: ""
  // Hint: 3-5 variant value must be exactly one grapheme cluster
  "",
  // Error: 3-16 invalid variant value: ""
  // Hint: 3-16 variant value must be exactly one grapheme cluster
  ("empty", "")
)

--- symbol-constructor-multi-cluster-variant-value eval ---
#symbol(
  // Error: 3-7 invalid variant value: "aa"
  // Hint: 3-7 variant value must be exactly one grapheme cluster
  "aa",
  // Error: 3-14 invalid variant value: "bb"
  // Hint: 3-14 variant value must be exactly one grapheme cluster
  ("b", "bb")
)

--- symbol-unknown-modifier eval ---
// Error: 13-20 unknown symbol modifier
#emoji.face.garbage

--- symbol-repr eval ---
#test(
  repr(sym.amp),
  `symbol("&", ("inv", "⅋"))`.text,
)
#test(
  repr(sym.amp.inv),
  `symbol("⅋")`.text,
)
#test(
  repr(sym.arrow.double.r),
  ```
  symbol(
    "⇒",
    ("bar", "⤇"),
    ("long", "⟹"),
    ("long.bar", "⟾"),
    ("not", "⇏"),
    ("struck", "⤃"),
    ("l", "⇔"),
    ("l.long", "⟺"),
    ("l.not", "⇎"),
    ("l.struck", "⤄"),
  )
  ```.text,
)
#test(repr(sym.smash), "symbol(\"⨳\")")

#let envelope = symbol(
  "🖂",
  ("stamped", "🖃"),
  ("stamped.pen", "🖆"),
  ("lightning", "🖄"),
  ("fly", "🖅"),
)
#test(
  repr(envelope),
  ```
  symbol(
    "🖂",
    ("stamped", "🖃"),
    ("stamped.pen", "🖆"),
    ("lightning", "🖄"),
    ("fly", "🖅"),
  )
  ```.text,
)
#test(
  repr(envelope.stamped),
  `symbol("🖃", ("pen", "🖆"))`.text,
)
#test(
  repr(envelope.stamped.pen),
  `symbol("🖆")`.text,
)
#test(
  repr(envelope.lightning),
  `symbol("🖄")`.text,
)
#test(
  repr(envelope.fly),
  `symbol("🖅")`.text,
)

--- issue-5930-symbol-label paged ---
#emoji.face<lab>
#context test(query(<lab>).first().text, "😀")

--- presentation-selectors paged ---
// Currently, presentation selectors do not cause font fallback when the main
// font supports at least one presentation, instead causing a fallback of the
// presentation form. This should probably be solved at some point, making the
// emojis below render with an emoji form.
// See: https://github.com/typst/typst/pull/6875.
#sym.copyright #emoji.copyright \
#sym.suit.heart #emoji.suit.heart

--- symbol-deprecation-warnings eval ---
// Test symbol deprecation warnings from codex.
#let pairs = (("emoji", emoji), ("sym", sym))
#let symbol-paths = ()

#while pairs.len() > 0 {
  let (path, sym-or-mod) = pairs.pop()
  let path-dot = path + "."

  if type(sym-or-mod) != symbol {
    pairs += dictionary(sym-or-mod)
      .pairs()
      .map(((name, s)) => (path-dot + name, s))
    continue
  }

  let mod-array = repr(sym-or-mod)
    .trim("symbol")
    // Make sure the closing paren has a comma to produce an array.
    .replace(regex("(,\\n)?\\)$"), ",)")

  let (maybe-mod-pair, ..modifier-pairs) = eval(mod-array)

  // The modifiers can start with a default symbol or a modifier pair.
  if type(maybe-mod-pair) == str {
    symbol-paths.push(path)
  } else {
    symbol-paths.push(path-dot + maybe-mod-pair.first())
  }
  // The rest are all modifier pairs.
  symbol-paths += modifier-pairs.map(((mod, _)) => path-dot + mod)
}

// Warning: 27-31 `spacebar` is deprecated, use `keyboard.space` instead
// Warning: 27-31 `succ.curly.eq` is deprecated, use `succ.eq.slant` instead
// Warning: 27-31 `prec.curly.eq` is deprecated, use `prec.eq.slant` instead
#let _ = symbol-paths.map(eval)
// This should be updated whenever the set of deprecated symbols updates.
//@@ file tests/suite/syntax_backtracking.typ
// Ensure that parser backtracking doesn't lead to exponential time consumption.
// If this regresses, the test suite will not terminate, which is a bit
// unfortunate compared to a good error, but at least we know something is up.
//

--- parser-backtracking-param-default-value eval ---
#{
  let s = "(x: 1) => x"
  let pat = "(x: {}) => 1 + x()"
  for _ in range(50) {
    s = pat.replace("{}", s)
  }
  test(eval(s)(), 51)
}

--- parser-backtracking-destructuring-assignment eval ---
#{
  let s = "(x) = 1"
  let pat = "(x: {_}) = 1"
  for _ in range(100) {
    s = pat.replace("_", s)
  }
  // Error: 8-9 cannot destructure integer
  eval(s)
}

--- parser-backtracking-destructuring-whitespace paged ---
// Test whitespace after memoized part.
#( (x: () => 1 ) => 1 )
//     -------
//     This is memoized and we want to ensure that whitespace after this
//     is handled correctly.
//@@ file tests/suite/syntax_code.typ
// Syntax tests for code mode and embedded code expressions.

--- embedded-code-incomplete eval ---
// Error: 2-2 expected expression
#

--- embedded-code-incomplete-followed-by-text eval ---
// Error: 2-2 expected expression
#  hello

--- embedded-code-extra-hash eval ---
// The span on the hints isn't great, but it's hard to fix.

// Error: 2-3 the character `#` is not valid in code
// Hint: 2-3 the preceding hash is causing this to parse in code mode
// Hint: 2-3 try escaping the preceding hash: `\#`
##

// Error: 2-3 the character `#` is not valid in code
// Hint: 2-3 the preceding hash is causing this to parse in code mode
// Hint: 2-3 try escaping the preceding hash: `\#`
// Error: 4-5 the character `#` is not valid in code
// Hint: 4-5 the preceding hash is causing this to parse in code mode
// Hint: 4-5 try escaping the preceding hash: `\#`
####

--- embedded-code-invalid-character eval ---
// Error: 2-3 the character `&` is not valid in code
// Hint: 2-3 the preceding hash is causing this to parse in code mode
// Hint: 2-3 try escaping the preceding hash: `\#`
#&&
// Error: 2-3 the character `!` is not valid in code
// Hint: 2-3 the preceding hash is causing this to parse in code mode
// Hint: 2-3 try escaping the preceding hash: `\#`
#!

--- code-syntax-extra-hash eval ---
// Test unneeded hashtags in code mode.
// Similar tests for markup exist in "syntax/embedded.typ"
#{
  // Error: 3-4 the character `#` is not valid in code
  // Hint: 3-4 you are already in code mode
  // Hint: 3-4 try removing the `#`
  #
  // Error: 3-4 the character `#` is not valid in code
  // Hint: 3-4 you are already in code mode
  // Hint: 3-4 try removing the `#`
  // Error: 4-5 the character `#` is not valid in code
  // Hint: 4-5 the preceding hash is causing this to parse in code mode
  // Hint: 4-5 try escaping the preceding hash: `\#`
  // Error: 5-6 the character `#` is not valid in code
  // Hint: 5-6 the preceding hash is causing this to parse in code mode
  // Hint: 5-6 try escaping the preceding hash: `\#`
  ###
}

--- code-syntax-bad-boolean-ops eval ---
// Test writing invalid boolean operators:
#{
  // Error: 3-5 `&&` is not valid in code
  // Hint: 3-5 in Typst, `and` is used for logical AND
  &&
  // Error: 3-4 the character `&` is not valid in code
  &
  // Error: 3-5 `||` is not valid in code
  // Hint: 3-5 in Typst, `or` is used for logical OR
  ||
  // Error: 3-4 the character `|` is not valid in code
  |
  // Error: 3-4 the character `!` is not valid in code
  // Hint: 3-4 in Typst, `not` is used for negation
  // Hint: 3-4 or did you mean to write `!=` for not-equal?
  !
  // Error: 3-4 the character `!` is not valid in code
  // Hint: 3-4 in Typst, `not` is used for negation
  // Hint: 3-4 or did you mean to write `!=` for not-equal?
  // Error: 5-6 unexpected equals sign
  ! =
  // Error: 3-4 the character `~` is not valid in code
  ~
  // Error: 3-5 `~=` is not valid in code
  // Hint: 3-5 in Typst, `!=` is used for not-equal
  ~=
}
//@@ file tests/suite/syntax_comment.typ
// Test line and block comments.

--- comments paged ---
// Line comment acts as spacing.
A// you
B

// Block comment does not act as spacing, nested block comments.
C/*
 /* */
*/D

// Works in code.
#test(type(/*1*/ 1) //
, int)

// End of block comment in line comment.
// Hello */

// Nested "//" doesn't count as line comment.
/* // */
E

/*//*/
This is a comment.
*/*/

--- comment-end-of-line paged ---
// Test comments at the end of a line
First part//
Second part

// Test comments at the end of a line with pre-spacing
First part          //
Second part

--- issue-4632-sth-followed-by-comment eval ---
// Test heading markers followed by comments.
#test([
  =// Comment
  =/* Comment */
], [
  =
  =
])

// Test list markers followed by comments.
#test([
  -// Comment
  -/* Comment */
], [
  -
  -
])

// Test enum markers followed by comments.
#test([
  +// Comment
  +/* Comment */

  1.// Comment
  2./* Comment */
], [
  +
  +

  1.
  2.
])


--- comment-block-unclosed eval ---
// End should not appear without start.
// Error: 7-9 unexpected end of block comment
// Hint: 7-9 consider escaping the `*` with a backslash or opening the block comment with `/*`
/* */ */

// Unterminated is okay.
/*
//@@ file tests/suite/syntax_depth.typ
--- parser-depth-exceeded-balanced eval ---
#{
  let s = "()"
  let pat = "({})"
  for _ in range(255) {
    s = pat.replace("{}", s)
  }
  // Ok
  eval(s)

  s = pat.replace("{}", s)
  // Error: 8-9 maximum parsing depth exceeded
  // Hint: 8-9 from index `256` to `258`
  eval(s)
}

--- parser-depth-exceeded-unbalanced eval ---
// Error: 7-17 unclosed delimiter
// Hint: 7-17 from index `0` to `1`
// Error: 7-17 maximum parsing depth exceeded
// Hint: 7-17 from index `256` to `1024`
#eval(1024 * "(")

--- parser-depth-exceeded-unbalanced-arrow eval ---
// https://issues.oss-fuzz.com/issues/42538221
// Error: 7-20 the character `#` is not valid in code
// Hint: 7-20 you are already in code mode
// Hint: 7-20 try removing the `#`
// Hint: 7-20 from index `0` to `1`
// Error: 7-20 unclosed delimiter
// Hint: 7-20 from index `1` to `2`
// Error: 7-20 unexpected arrow
// Hint: 7-20 from index `3` to `5`
// Error: 7-20 maximum parsing depth exceeded
// Hint: 7-20 from index `641` to `2560`
#eval(512 * "#((=>")

--- parser-depth-exceeded-unop eval ---
// https://issues.oss-fuzz.com/issues/415163163
// Error: 7-17 maximum parsing depth exceeded
// Hint: 7-17 from index `512` to `513`
// Error: 7-17 expected expression
// Hint: 7-17 at index `1023`
#eval(512 * "- ")
//@@ file tests/suite/syntax_escape.typ
// Test escape sequences.

--- escape paged ---
// Escapable symbols.
\\ \/ \[ \] \{ \} \# \* \_ \+ \= \~ \
\` \$ \" \' \< \> \@ \( \) \A

// No need to escape.
( ) ;

// Escaped comments.
\//
\/\* \*\/
\/* \*/ *

// Unicode escape sequence.
\u{1F3D5} == 🏕

// Escaped escape sequence.
\u{41} vs. \\u\{41\}

// Some code stuff in text.
let f() , ; : | + - /= == 12 "string"

// Escaped dot.
10\. May

--- escape-invalid-codepoint eval ---
// Unicode codepoint does not exist.
// Error: 1-11 invalid Unicode codepoint: FFFFFF
\u{FFFFFF}

--- escape-unclosed eval ---
// Unterminated.
// Error: 1-6 unclosed Unicode escape sequence
\u{41[*Bold*]
//@@ file tests/suite/syntax_newlines.typ
// Test newline continuations.

--- newline-continuation-code paged ---
#{
  "hello"
    .clusters()
  if false {

  }
  else {
    ("1", "2")
  }
}

--- newline-continuation-markup paged ---
#"hello"
  .codepoints()

#if false {

}
else {
  ("1", "2")
}

--- newline-continuation-method-blank eval ---
#test({
  "hi 1"

    .clusters()
}, ("h", "i", " ", "1"))

--- newline-continuation-method-line-comment-after eval ---
#test({
  "hi 2"// comment
    .clusters()
}, ("h", "i", " ", "2"))

--- newline-continuation-method-block-comment-after eval ---
#test({
  "hi 3"/* comment */
    .clusters()
}, ("h", "i", " ", "3"))

--- newline-continuation-method-line-comment-between eval ---
#test({
  "hi 4"
  // comment
    .clusters()
}, ("h", "i", " ", "4"))

--- newline-continuation-method-block-comment-between eval ---
#test({
  "hi 5"
  /*comment*/.clusters()
}, ("h", "i", " ", "5"))

--- newline-continuation-method-comments-and-blanks eval ---
#test({
  "hi 6"
  // comment


  /* comment */
    .clusters()
}, ("h", "i", " ", "6"))

--- newline-continuation-if-else-comment eval ---
#test({
  let foo(x) = {
    if x < 0 { "negative" }
    // comment
    else { "non-negative" }
  }

  foo(1)
}, "non-negative")
//@@ file tests/suite/syntax_numbers.typ
// Test how numbers are displayed.

--- numbers paged ---
// Test numbers in text mode.
#set text(font: ("Libertinus Serif", "Noto Sans Arabic"))
12 \
12.0 \
3.14 \
1234567890 \
0123456789 \
0 \
0.0 \
+0 \
+0.0 \
-0 \
-0.0 \
-1 \
-3.14 \
-9876543210 \
-0987654321 \
٣٫١٤ \
-٣٫١٤ \
-¾ \
#text(fractions: true)[-3/2] \
2022 - 2023 \
2022 -- 2023 \
2022--2023 \
2022-2023 \
٢٠٢٢ - ٢٠٢٣ \
٢٠٢٢ -- ٢٠٢٣ \
٢٠٢٢--٢٠٢٣ \
٢٠٢٢-٢٠٢٣ \
-500 -- -400
//@@ file tests/suite/syntax_panics.typ
// Tests for panics in the parser caused by syntax.

--- issue-4571-panic-when-compiling-invalid-file eval ---
// Test that trying to parse the following does not result in a panic.

// Error: 9-10 unclosed delimiter
// Error: 22 expected pattern
// Error: 23-24 unexpected star
#import (hntle-clues: *
// Error: 1-2 the character `#` is not valid in code
// Hint: 1-2 you are already in code mode
// Hint: 1-2 try removing the `#`
// Error: 2-8 expected pattern, found keyword `import`
// Hint: 2-8 keyword `import` is not allowed as an identifier; try `import_` instead
// Error: 9-20 expected identifier, found string
#import "/util.typ": qrlink
// Error: 1-2 the character `#` is not valid in code
// Hint: 1-2 you are already in code mode
// Hint: 1-2 try removing the `#`
// Error: 2-5 expected pattern, found keyword `let`
// Hint: 2-5 keyword `let` is not allowed as an identifier; try `let_` instead
#let auton(
// Error: 3-4 unexpected equals sign
// Error: 5-6 unclosed delimiter
// Error: 6 expected equals sign
) = {
//@@ file tests/suite/syntax_shebang.typ
// Test shebang support.

--- shebang eval ---
#!typst compile

// Error: 2-3 the character `!` is not valid in code
// Hint: 2-3 the preceding hash is causing this to parse in code mode
// Hint: 2-3 try escaping the preceding hash: `\#`
#!not-a-shebang
//@@ file tests/suite/syntax_shorthand.typ
// Test shorthands for unicode codepoints.

--- shorthand-nbsp-and-shy-hyphen paged ---
The non-breaking space~does work, soft-?hyphen also does.

--- shorthand-nbsp-width paged ---
// Make sure non-breaking and normal space always
// have the same width. Even if the font decided
// differently.
#set text(font: "New Computer Modern")
a b \
a~b

--- shorthand-dashes paged ---
- En dash: --
- Em dash: ---

--- shorthand-ellipsis paged ---
#set text(font: "Roboto")
A... vs #"A..."

--- shorthand-minus paged ---
// Make sure shorthand is applied only before a digit.
-a -1

--- shorthands-math paged ---
// Check all math shorthands.
$...$\
$-$\
$'$\
$*$\
$~$\
$!=$\
$:=$\
$::=$\
$=:$\
$<<$\
$<<<$\
$>>$\
$>>>$\
$<=$\
$>=$\
$->$\
$-->$\
$|->$\
$>->$\
$->>$\
$<-$\
$<--$\
$<-<$\
$<<-$\
$<->$\
$<-->$\
$~>$\
$~~>$\
$<~$\
$<~~$\
$=>$\
$|=>$\
$==>$\
$<==$\
$<=>$\
$<==>$\
$[|$\
$|]$\
$||$
//@@ file tests/suite/text_case.typ
// Test the `upper` and `lower` functions.

--- lower-and-upper eval ---
#let memes = "ArE mEmEs gReAt?";
#test(lower(memes), "are memes great?")
#test(upper(memes), "ARE MEMES GREAT?")
#test(upper("Ελλάδα"), "ΕΛΛΆΔΑ")

--- cases-content-text paged ---
// Check that cases are applied to text nested in content
#lower(box("HI!"))

--- cases-content-symbol paged ---
// Check that cases are applied to symbols nested in content
#lower($H I !$.body)

--- cases-content-html html ---
#lower[MY #html.strong[Lower] #symbol("A")] \
#upper[my #html.strong[Upper] #symbol("a")] \

--- upper-bad-type eval ---
// Error: 8-9 expected string or content, found integer
#upper(1)
//@@ file tests/suite/text_coma.typ
--- coma paged large ---
#set page(width: 450pt, margin: 1cm)

*Technische Universität Berlin* #h(1fr) *WiSe 2019/2020* \
*Fakultät II, Institut for Mathematik* #h(1fr) Woche 3 \
Sekretariat MA \
Dr. Max Mustermann \
Ola Nordmann, John Doe

#v(3mm)
#align(center)[
  #set par(leading: 3mm)
  #text(1.2em)[*3. Übungsblatt Computerorientierte Mathematik II*] \
  *Abgabe: 03.05.2019* (bis 10:10 Uhr in MA 001) \
  *Alle Antworten sind zu beweisen.*
]

*1. Aufgabe* #h(1fr) (1 + 1 + 2 Punkte)

Ein _Binärbaum_ ist ein Wurzelbaum, in dem jeder Knoten ≤ 2 Kinder hat.
Die Tiefe eines Knotens _v_ ist die Länge des eindeutigen Weges von der Wurzel
zu _v_, und die Höhe von _v_ ist die Länge eines längsten (absteigenden) Weges
von _v_ zu einem Blatt. Die Höhe des Baumes ist die Höhe der Wurzel.

#align(center, image("/assets/images/graph.png", width: 75%))
//@@ file tests/suite/text_copy-paste.typ
// Test copy-paste and search in PDF with ligatures
// and Arabic test. Must be tested manually!

--- text-copy-paste-ligatures paged ---
The after fira 🏳️‍🌈!

#set text(lang: "ar", font: "Noto Sans Arabic")
مرحبًا
//@@ file tests/suite/text_deco.typ
// Test text decorations.

--- underline-overline-strike paged ---
#let red = rgb("fc0030")

// Basic strikethrough.
#strike[Statements dreamt up by the utterly deranged.]

// Move underline down.
#underline(offset: 5pt)[Further below.]

// Different color.
#underline(stroke: red, evade: false)[Critical information is conveyed here.]

// Inherits font color.
#text(fill: red, underline[Change with the wind.])

// Both over- and underline.
#overline(underline[Running amongst the wolves.])

--- strike-with paged ---
#let redact = strike.with(stroke: 10pt, extent: 0.05em)
#let highlight-custom = strike.with(stroke: 10pt + rgb("abcdef88"), extent: 0.05em)

// Abuse thickness and transparency for redacting and highlighting stuff.
Sometimes, we work #redact[in secret].
There might be #highlight-custom[redacted] things.

--- underline-stroke-folding paged ---
// Test stroke folding.
#set underline(stroke: 2pt, offset: 2pt)
#underline(text(red, [DANGER!]))

--- underline-background paged ---
// Test underline background
#set underline(background: true, stroke: (thickness: 0.5em, paint: red, cap: "round"))
#underline[This is in the background]

--- overline-background paged ---
// Test overline background
#set overline(background: true, stroke: (thickness: 0.5em, paint: red, cap: "round"))
#overline[This is in the background]

--- strike-background paged ---
// Test strike background
#set strike(background: true, stroke: 5pt + red)
#strike[This is in the background]

--- highlight paged ---
// Test highlight.
This is the built-in #highlight[highlight with default color].
We can also specify a customized value
#highlight(fill: green.lighten(80%))[to highlight].

--- highlight-bounds paged ---
// Test default highlight bounds.
#highlight[ace],
#highlight[base],
#highlight[super],
#highlight[phone #sym.integral]

--- highlight-edges paged ---
// Test a tighter highlight.
#set highlight(top-edge: "x-height", bottom-edge: "baseline")
#highlight[ace],
#highlight[base],
#highlight[super],
#highlight[phone #sym.integral]

--- highlight-edges-bounds paged ---
// Test a bounds highlight.
#set highlight(top-edge: "bounds", bottom-edge: "bounds")
#highlight[abc]
#highlight[abc #sym.integral]

--- highlight-radius paged ---
// Test highlight radius
#highlight(radius: 3pt)[abc],
#highlight(radius: 1em)[#lorem(5)]

--- highlight-stroke paged ---
// Test highlight stroke
#highlight(stroke: 2pt + blue)[abc]
#highlight(stroke: (top: blue, left: red, bottom: green, right: orange))[abc]
#highlight(stroke: 1pt, radius: 3pt)[#lorem(5)]

--- html-deco html ---
#strike[Struck]
#highlight[Highlighted]
#underline[Underlined]
#overline[Overlined]

#(strike, highlight, underline, overline).fold([Mixed], (it, f) => f(it))
//@@ file tests/suite/text_edge.typ
// Test top and bottom text edge.

--- text-edge paged ---
#set page(width: 160pt)
#set text(size: 8pt)

#let try(top, bottom) = rect(inset: 0pt, fill: conifer)[
  // Warning: 19-34 unknown font family: ibm plex mono
  #set text(font: "IBM Plex Mono", top-edge: top, bottom-edge: bottom)
  From #top to #bottom
]

#let try-bounds(top, bottom) = rect(inset: 0pt, fill: conifer)[
  // Warning: 19-34 unknown font family: ibm plex mono
  #set text(font: "IBM Plex Mono", top-edge: top, bottom-edge: bottom)
  #top to #bottom: "yay, Typst"
]

#try("ascender", "descender")
#try("ascender", "baseline")
#try("cap-height", "baseline")
#try("x-height", "baseline")
#try-bounds("cap-height", "baseline")
#try-bounds("bounds", "baseline")
#try-bounds("bounds", "bounds")
#try-bounds("x-height", "bounds")

#try(4pt, -2pt)
#try(1pt + 0.3em, -0.15em)

--- text-edge-bad-type eval ---
// Error: 21-23 expected "ascender", "cap-height", "x-height", "baseline", "bounds", or length, found array
#set text(top-edge: ())

--- text-edge-bad-value eval ---
// Error: 24-26 expected "baseline", "descender", "bounds", or length
#set text(bottom-edge: "")

--- text-edge-wrong-edge eval ---
// Error: 24-36 expected "baseline", "descender", "bounds", or length
#set text(bottom-edge: "cap-height")
//@@ file tests/suite/text_em.typ
// Test font-relative sizing.

--- text-size-em-nesting paged ---
#set text(size: 5pt)
A // 5pt
#[
  #set text(size: 2em)
  B // 10pt
  #[
    #set text(size: 1.5em + 1pt)
    C // 16pt
    #text(size: 2em)[D] // 32pt
    E // 16pt
  ]
  F // 10pt
]
G // 5pt

--- text-size-em paged ---
// Test using ems in arbitrary places.
#set text(size: 5pt)
#set text(size: 2em)
#set square(fill: red)

#let size = {
  let size = 0.25em + 1pt
  for _ in range(3) {
    size *= 2
  }
  size - 3pt
}

#stack(dir: ltr, spacing: 1fr, square(size: size), square(size: 25pt))
//@@ file tests/suite/text_font.typ
// Test configuring font properties.

--- text-font-properties paged ---
// Set same font size in three different ways.
#text(20pt)[A]
#text(2em)[A]
#text(size: 15pt + 0.5em)[A]

// Do nothing.
#text()[Normal]

// Set style (is available).
#text(style: "italic")[Italic]

// Set weight (is available).
#text(weight: "bold")[Bold]

// Set stretch (not available, matching closest).
#text(stretch: 50%)[Condensed]

// Set font family.
#text(font: "IBM Plex Serif")[Serif]

// Emoji.
Emoji: 🐪, 🌋, 🏞

// Colors.
#[
  #set text(fill: eastern)
  This is #text(rgb("FA644B"))[way more] colorful.
]

// Transparency.
#block(fill: green)[
  #set text(fill: rgb("FF000080"))
  This text is transparent.
]

// Disable font fallback beyond the user-specified list.
// Without disabling, New Computer Modern Math would come to the rescue.
#set text(font: ("PT Sans", "Twitter Color Emoji"), fallback: false)
2π = 𝛼 + 𝛽. ✅

--- text-call-body paged ---
// Test string body.
#text("Text") \
#text(red, "Text") \
#text(font: "Ubuntu", blue, "Text") \
#text([Text], teal, font: "IBM Plex Serif") \
#text(forest, font: "New Computer Modern", [Text]) \

--- text-bad-argument eval ---
// Error: 11-16 unexpected argument
#set text(false)

--- text-style-bad eval ---
// Error: 18-24 expected "normal", "italic", or "oblique"
#set text(style: "bold", weight: "thin")

--- text-bad-extra-argument eval ---
// Error: 23-27 unexpected argument
#set text(size: 10pt, 12pt)

--- text-bad-named-argument eval ---
// Error: 11-31 unexpected argument: something
#set text(something: "invalid")

--- text-unknown-font-family-warning paged ---
#text(font: "libertinus serif")[I exist,]
// Warning: 13-26 unknown font family: nonexistent
#text(font: "nonexistent")[but]
// Warning: 17-35 unknown font family: also-nonexistent
#set text(font: "also-nonexistent")
I
// Warning: 23-55 unknown font family: list-of
// Warning: 23-55 unknown font family: nonexistent-fonts
#let var = text(font: ("list-of", "nonexistent-fonts"))[don't]
#var

--- issue-5499-text-fill-in-clip-block paged ---

#let t = tiling(
  size: (30pt, 30pt),
  relative: "parent",
  square(
    size: 30pt,
    fill: gradient
      .conic(..color.map.rainbow),
  )
)

#block(clip: false, height: 2em, {
  text(fill: blue, "Hello")
  [ ]
  text(fill: blue.darken(20%).transparentize(50%), "Hello")
  [ ]
  text(fill: gradient.linear(..color.map.rainbow), "Hello")
  [ ]
  text(fill: t, "Hello")
})
#block(clip: true, height: 2em, {
  text(fill: blue, "Hello")
  [ ]
  text(fill: blue.darken(20%).transparentize(50%), "Hello")
  [ ]
  text(fill: gradient.linear(..color.map.rainbow), "Hello")
  [ ]
  text(fill: t, "Hello")
})

--- text-font-types paged empty ---
#let ubuntu = (name: "Ubuntu", covers: regex("[\u{20}-\u{FFFF}]"))
#set text(font: ubuntu)
#set text(font: (ubuntu, "Ubuntu"))

--- text-font-covers-chinese paged ---
// Without ranges, the quotation mark is using the Latin font.
#set text(font: ("Ubuntu", "Noto Serif CJK SC"))
分别设置“中文”和English字体

// With ranges, the quotation mark is using the Chinese font.
#set text(font: ((name: "Noto Serif CJK SC", covers: regex("[\u{00B7}-\u{3134F}]")), "Ubuntu"))
分别设置“中文”和English字体

// With "latin-in-cjk", the quotation mark is also using the Chinese font.
#set text(font: ((name: "Ubuntu", covers: "latin-in-cjk"), "Noto Serif CJK SC"))
分别设置“中文”和English字体

--- text-font-covers-numbers paged ---
// Change font only for numbers.
#set text(font: (
  (name: "PT Sans", covers: regex("[0-9]")),
  "Libertinus Serif"
))

The number 123.

--- text-font-covers-repeat paged ---
// Repeatedly use the same font.
#set text(font: (
  (name: "Libertinus Serif", covers: regex("[0-9]")),
  "Libertinus Serif"
))

The number 123.

--- text-font-covers-riffle paged ---
// Repeatedly use two fonts alternately.
#set text(font: (
  (name: "Noto Color Emoji", covers: regex("[🔗⛓‍💥]")),
  (name: "Twitter Color Emoji", covers: regex("[^🖥️]")),
  "Noto Color Emoji",
))

🔗⛓‍💥🖥️🔑

// The above should be the same as:
#{
  text(font: "Noto Color Emoji", "🔗⛓‍💥🖥️")
  text(font: "Twitter Color Emoji", "🔑")
}

// but not:
#text(font: "Twitter Color Emoji", "🔗⛓‍💥🖥️🔑")

--- text-colr-svg-glyphs-different-size paged ---
#set text(size: 11pt)
#text(font: "Noto Color Emoji", "🔗⛓‍💥🖥️🔑") \
#text(font: "Twitter Color Emoji", "🔗⛓‍💥🖥️🔑") \

#set text(size: 22pt)
#text(font: "Noto Color Emoji", "🔗⛓‍💥🖥️🔑") \
#text(font: "Twitter Color Emoji", "🔗⛓‍💥🖥️🔑") \

--- text-font-covers-bad-1 eval ---
// Error: 17-59 coverage regex may only use dot, letters, and character classes
// Hint: 17-59 the regex is applied to each letter individually
#set text(font: (name: "Ubuntu", covers: regex("20-FFFF")))

--- text-font-covers-bad-2 eval ---
// Error: 17-65 coverage regex may only use dot, letters, and character classes
// Hint: 17-65 the regex is applied to each letter individually
#set text(font: (name: "Ubuntu", covers: regex("\u{20}-\u{10}")))

--- text-font-covers-reflection paged empty ---
// reflect "latin-in-cjk" covers
#set text(font: (name: "Ubuntu", covers: "latin-in-cjk"))
#context test(text.font, (name: "ubuntu", covers: "latin-in-cjk"))

// reflect regex covers
#set text(font: (name: "Ubuntu", covers: regex("\d")))
#context test(text.font, (name: "ubuntu", covers: regex("\d")))

// reflect font list with covers
#set text(font: ((name: "Ubuntu", covers: regex("\d")), "IBM Plex Serif"))
#context test(text.font, ((name: "ubuntu", covers: regex("\d")), "ibm plex serif"))

--- issue-5262-text-negative-size paged ---
#set text(-1pt)

a

--- issue-5940-text-negative-size-panic paged ---
#set align(center)
#set text(-10pt)
Hello

--- empty-text-font-array eval ---
// Error: 17-19 font fallback list must not be empty
#set text(font: ())

--- text-font-variable-ital paged ---
#set text(font: "Mona Sans")
Hello _Hello_

#text(variations: (ital: 0))[Hello]
#text(variations: (ital: 1))[Hello]

--- text-font-variable-slnt paged ---
#set page(width: auto)
#set text(font: "Roboto Flex")

Hello, _Hello_

#text(style: "italic")[Hello],
#text(style: "oblique")[Hello]

#for slnt in range(0, -10, step: -2, inclusive: true) [
  #text(variations: (slnt: slnt))[Hello.]
]

--- text-font-variable-wght paged ---
#set page(width: auto)
#for (font, tech) in (("Fraunces", "TTF"), ("Cantarell", "CFF2")) [
  #set text(font: "Fraunces")
  = #tech

  Hello, *Hello*

  #for weight in range(200, 900, step: 100, inclusive: true) [
    #text(weight: weight)[Hello.]
  ]

  #for weight in range(200, 900, step: 100, inclusive: true) [
    #text(variations: (wght: weight))[Hello.]
  ]
]

--- text-font-variable-wdth paged ---
#set page(width: auto)
#set text(font: "Roboto Flex")

Hello

#for stretch in range(50, 150, step: 10) [
  #text(stretch: stretch * 1%)[Hello.]
]

#for stretch in range(50, 150, step: 10) [
  #text(variations: (wdth: stretch))[Hello.]
]

--- text-font-variable-opsz paged ---
#set page(width: auto)
#set text(font: "Fraunces")

#for base in (10pt, 20pt) {
  for s in range(1, 5, inclusive: true) [
    #let scaled = s * base
    #scale(100% / s, reflow: true, text(size: scaled)[Hello])
  ]
}

--- text-font-variable-custom-wonk paged ---
#set page(width: auto)
#set text(font: "Fraunces", size: 25pt)

// WONK only kicks in at point sizes > 18pt.
#text(variations: (WONK: 0))[minimum] \
minimum \
#text(variations: (WONK: 1))[minimum]

--- text-font-variable-custom-grad paged ---
#set page(width: auto)
#set text(font: "Roboto Flex")

#text(variations: (GRAD: -200))[Grade] axis \
Grade axis \
#text(variations: (GRAD: 150))[Grade] axis

--- text-font-variable-custom-soft paged ---
// Soft axis becomes more visible at large font size, so we increase it and then
// scale down to avoid a huge test image.
#set text(font: "Fraunces", size: 100pt)
#scale(20%, reflow: true)[
  #set text(variations: (SOFT: 0))
  Soft?
  #set text(variations: (SOFT: 100))
  Soft!
]

--- text-font-variable-and-static paged ---
// This font exists both in its static and variable version.
#set text(font: "Source Serif 4")
Hello _world_ *with* #text(weight: 550)[_Source Serif._]

--- text-font-variable-multiple paged ---
// Multiple fonts with multiple different axis combinations in one test.
#text(font: "Roboto Flex")[
  Roboto _Flex_
  #text(variations: (GRAD: 150))[
    with #text(stretch: 150%)[*Grade* axis] enabled
  ]
] \
#text(font: "Source Serif 4")[
  Source _Serif_ 4 *Variable*
]

--- text-font-variations-win paged ---
// Test that custom variations win over built-in settings.
#set text(font: "Mona Sans")
#text(style: "italic")[
  Italic \
  #text(variations: (ital: 0))[Not italic]
]

--- text-font-variations-fold paged empty ---
#set text(variations: (ital: 1, GRAD: 10))
#set text(variations: (GRAD: 15))
#context test(text.variations, (ital: 1, GRAD: 15))

--- text-font-variations-invalid eval ---
// Error: 23-34 tag must be one to four characters in length
// Hint: 23-34 found 5 characters
// Hint: 23-34 occurred in tag at index 0 (`"grade"`)
#set text(variations: (grade: 10))
//@@ file tests/suite/text_lang.typ
// Test setting the document language.

--- text-lang paged html pdftags ---
#set pdf(standard: "ua-1")
// without any region
#set text(font: "Noto Serif CJK TC", lang: "zh")
#outline()

--- text-lang-unknown-region paged html pdftags ---
#set pdf(standard: "ua-1")
// with unknown region configured
#set text(font: "Noto Serif CJK TC", lang: "zh", region: "XX")
#outline()

--- text-lang-region paged html pdftags ---
#set pdf(standard: "ua-1")
// with region configured
#set text(font: "Noto Serif CJK TC", lang: "zh", region: "TW")
#outline()

--- text-lang-hyphenate paged ---
// Ensure that setting the language does have effects.
#set text(hyphenate: true)
#grid(
  columns: 2 * (20pt,),
  gutter: 1fr,
  text(lang: "en")["Eingabeaufforderung"],
  text(lang: "de")["Eingabeaufforderung"],
)

--- text-lang-shaping paged ---
// Test that the language passed to the shaper has an effect.
#set text(font: "Ubuntu")

// Some lowercase letters are different in Serbian Cyrillic compared to other
// Cyrillic languages. Since there is only one set of Unicode codepoints for
// Cyrillic, these can only be seen when setting the language to Serbian and
// selecting one of the few fonts that support these letterforms.
Бб
#text(lang: "uk")[Бб]
#text(lang: "sr")[Бб]

--- text-lang-script-shaping paged ---
// Verify that writing script/language combination has an effect
#{
  set text(size:20pt)
  set text(script: "latn", lang: "en")
  [Ş ]
  set text(script: "latn", lang: "ro")
  [Ş ]
  set text(script: "grek", lang: "ro")
  [Ş ]
}

--- text-script-bad-type eval ---
// Error: 19-23 expected string or auto, found none
#set text(script: none)

--- text-script-bad-value eval ---
// Error: 19-23 expected three or four letter script code (ISO 15924 or 'math')
#set text(script: "ab")

--- text-lang-bad-type eval ---
// Error: 17-21 expected string, found none
#set text(lang: none)

--- text-lang-bad-value eval ---
// Error: 17-20 expected two or three letter language code (ISO 639-1/2/3)
#set text(lang: "ӛ")

--- text-lang-bad-value-emoji eval ---
// Error: 17-20 expected two or three letter language code (ISO 639-1/2/3)
#set text(lang: "😃")

--- text-region-bad-value eval ---
// Error: 19-24 expected two letter region code (ISO 3166-1 alpha-2)
#set text(region: "hey")

--- text-language-fallback-english paged ---
#set text(lang: "qaa")
#outline()
#set text(lang: "qaa", region: "aa")
#outline()

--- text-lang-hint-region-parameter eval ---
// Error: 17-24 expected two or three letter language code (ISO 639-1/2/3)
// Hint: 17-24 you should leave only "en" in the `lang` parameter and specify "gb" in the `region` parameter
#set text(lang: "en-gb")
//@@ file tests/suite/text_lorem.typ
// Test blind text.

--- lorem paged ---
// Test basic call.
#lorem(19)

--- lorem-pars paged ---
// Test custom paragraphs with user code.
#set text(8pt)

#{
  let sentences = lorem(59)
    .split(".")
    .filter(s => s != "")
    .map(s => s + ".")

  let used = 0
  for s in sentences {
    if used < 2 {
      used += 1
    } else {
      parbreak()
      used = 0
    }
    s.trim()
    [ ]
  }
}

--- lorem-missing-words eval ---
// Error: 2-9 missing argument: words
#lorem()

--- lorem-word-count eval ---
// https://github.com/typst/typst/issues/6186
#let count(n) = lorem(n).replace("–", "").replace(".", "").split(" ").filter(s => s != "").len()
#test(count(193), 193)
#test(count(194), 194)
#test(count(195), 195)
//@@ file tests/suite/text_raw.typ
// Test raw blocks.

--- raw-empty paged ---
// Empty raw block.
Empty raw block:``.

--- raw-consecutive-single-backticks paged ---
// No extra space.
`A``B`

--- raw-typst-lang paged ---
// Typst syntax inside.
```typ #let x = 1``` \
```typ #f(1)```

--- raw-block-no-parbreaks paged ---
// Multiline block splits paragraphs.

Text
```rust
fn code() {}
```
Text

--- raw-more-backticks paged ---
// Lots of backticks inside.
````
```backticks```
````

--- raw-trimming paged ---
// Trimming.

// Space between "rust" and "let" is trimmed.
The keyword ```rust let```.

// Trimming depends on number backticks.
(``) \
(` untrimmed `) \
(``` trimmed` ```) \
(``` trimmed ```) \
(``` trimmed```) \

--- raw-single-backtick-lang paged ---
// Single ticks should not have a language.
`rust let`

--- raw-dedent-first-line paged ---
// First line is not dedented and leading space is still possible.
     ```   A
        B
       C
     ```

--- raw-dedent-empty-line paged ---
// Do not take empty lines into account when computing dedent.
```
        A

        B
```

--- raw-dedent-last-line paged ---
// Take last line into account when computing dedent.
```
        A

        B
    ```

--- raw-empty-inline eval ---
#let raw = ``
#test(raw.text, "")
#test(raw.block, false)

--- raw-empty-spaces eval ---
#let raw = ```   ```
#test(raw.text, "")
#test(raw.block, false)

--- raw-empty-newlines eval ---
#let raw = ```


```
#test(raw.text, "\n")
#test(raw.block, true)

--- raw-newlines-backtick eval ---
#let raw = ```

`

```
#test(raw.text, "\n`\n")
#test(raw.block, true)

--- raw-backtick eval ---
#let raw = ``` ` ```
#test(raw.text, "`")
#test(raw.block, false)

--- raw-lang-backtick eval ---
#let raw = ```lang ` ```
#test(raw.lang, "lang")
#test(raw.text, "`")
#test(raw.block, false)

--- raw-lang-backtick-no-space eval ---
// The language tag stops at a backtick even without whitespace.
// TODO: Do we want this behavior? It was not discussed in #7337.
#let raw = ```lang`test ` ```
#test(raw.lang, "lang")
#test(raw.text, "`test `")
#test(raw.block, false)

--- raw-lang-space eval ---
// The language tag stops at a space.
#let raw = ```lang test ```
#test(raw.lang, "lang")
#test(raw.text, "test ")
#test(raw.block, false)

--- raw-lang--multi-space eval ---
// The language tag only discards one space.
#let raw = ```lang  test```
#test(raw.lang, "lang")
#test(raw.text, " test")
#test(raw.block, false)

--- raw-lang-newline eval ---
// The language tag stops at a newline.
#let raw = ```lang
test
```
#test(raw.lang, "lang")
#test(raw.text, "test")
#test(raw.block, true)

--- raw-lang-no-text eval ---
// Warning: 12-22 empty raw text
// Hint: 12-22 Typst is treating `lang` as the language tag
// Hint: 15-19 to treat this as text, add a space after the initial backticks
#let raw = ```lang```
#test(raw.lang, "lang")
#test(raw.text, "")
#test(raw.block, false)

--- raw-lang-non-ident eval ---
// The language tag does not have to be a valid identifier.
// Warning: 15-25 no whitespace between language tag and raw text
// Hint: 15-19 if the current behavior is correct, please add a space after `lang`
// Hint: 15-19 otherwise, add a space or newline after the initial backticks
// Hint: 15-25 currently, Typst is treating `lang` as the language tag
// Hint: 15-25 in the next version of Typst, this will change and we will treat all text until the first whitespace as the language tag
#let raw = ```lang.tag++ test```
#test(raw.lang, "lang")
#test(raw.text, ".tag++ test")
#test(raw.block, false)

--- raw-lang-starts-non-ident eval ---
// Test the language tag starting with non-identifier characters.
// Warning: 15-31 no whitespace before raw text
// Hint: 15-31 in the next version of Typst, this text will be treated as the language tag for this element
// Hint: 15-31 to avoid this, add a space after the initial backticks
#let raw = ```!@#$%^&*()_+lang test```
#test(raw.text, "!@#$%^&*()_+lang test")
// Error: 11-15 field "lang" in raw is not known at this point
#test(raw.lang, none)
#test(raw.block, false)

--- raw-blocky eval ---
// The first line and the last line are ignored.
#let raw = {
```
test
```
}
#test(raw.text, "test")
#test(raw.block, true)

--- raw-blocky-dedent eval ---
// A blocky raw should handle dedents.
#let raw = {
```
test
```
}
#test(raw.text, "test")
#test(raw.block, true)

--- raw-blocky-dedent-firstline eval ---
// When there is content in the first line, we discard a single whitespace char.
#let raw = ``` test
  ```
#test(raw.text, "test")
#test(raw.block, true)

--- raw-blocky-dedent-firstline2 eval ---
// When there is content in the first line, we discard a single whitespace char.
#let raw = ``` test
```
#test(raw.text, "test")
#test(raw.block, true)

--- raw-blocky-dedent-firstline3 eval ---
// The first line is not affected by dedent, and the middle lines don't consider
// the whitespace prefix of the first line.
#let raw = ``` test
     test2
  ```
#test(raw.text, "test\n   test2")
#test(raw.block, true)

--- raw-blocky-dedent-firstline4 eval ---
// The first line is not affected by dedent, and the middle lines don't consider
// the whitespace prefix of the first line.
#let raw = ```     test
  test2
  ```
#test(raw.text, "    test\ntest2")
#test(raw.block, true)

--- raw-blocky-dedent-lastline eval ---
#let raw = ```
  test
 ```
#test(raw.text, " test")
#test(raw.block, true)

--- raw-blocky-dedent-lastline2 eval ---
#let raw = ```
  test
  ```
#test(raw.text, "test")
#test(raw.block, true)

--- raw-blocky-tab eval ---
#let raw = {
```
	test
```
}
#test(raw.text, "\ttest")
#test(raw.block, true)

--- raw-blocky-tab-dedent eval ---
// This one is a bit problematic because there is a trailing tab below "test"
// which the editor constantly wants to remove.
#let raw = eval("```\n\ttest\n  \n ```")
#test(raw.text, "test\n ")
#test(raw.block, true)

--- raw-extra-first-line-ws eval ---
#let raw = eval("```   \n```")
#test(raw.text, "")
#test(raw.block, true)

--- raw-tab-size paged ---
#set raw(tab-size: 8)

```tsv
Year	Month	Day
2000	2	3
2001	2	1
2002	3	10
```

--- raw-syntaxes paged ---
#set page(width: 180pt)
#set text(6pt)
#set raw(syntaxes: "/assets/syntaxes/SExpressions.sublime-syntax")

```sexp
(defun factorial (x)
  (if (zerop x)
    ; with a comment
    1
    (* x (factorial (- x 1)))))
```

--- raw-syntaxes-invalid-sublime-syntax eval ---
// Prevent test parser from failing on "^---" line.
#let sublime-syntax = ```yaml
%YAML 1.2
```.text + "\n---\n" + ```yaml
name: lang
file_extensions:
  - a
scope: source
contexts:
  main:
    - match: '\'
```.text

// Error: 35-56 failed to parse syntax (Error while compiling regex '\': Parsing error at position 0: Backslash without following character)
#raw("text", lang: "a", syntaxes: bytes(sublime-syntax))

--- raw-syntaxes-types paged empty ---
#let sublime-syntax = ```yaml
%YAML 1.2
```.text + "\n---\n" + ```yaml
name: lang
file_extensions:
  - a
scope: source
contexts:
  main:
    - match: ''
```.text

#set raw(syntaxes: "/assets/syntaxes/SExpressions.sublime-syntax")
#set raw(syntaxes: path("/assets/syntaxes/SExpressions.sublime-syntax"))
#set raw(syntaxes: (
  path("/assets/syntaxes/SExpressions.sublime-syntax"),
  bytes(sublime-syntax),
))

--- raw-theme paged ---
// Test code highlighting with custom theme.
#set page(width: 180pt)
#set text(6pt)
#set raw(theme: "/assets/themes/halcyon.tmTheme")
#show raw: it => {
  set text(fill: rgb("a2aabc"))
  rect(
    width: 100%,
    inset: (x: 4pt, y: 5pt),
    radius: 4pt,
    fill: rgb("1d2433"),
    place(right, text(luma(240), it.lang)) + it,
  )
}

```typ
= Chapter 1
#lorem(100)

#let hi = "Hello World"
#show heading: emph
```

--- raw-show-set paged ---
// Text show rule
#show raw: set text(font: "Roboto")
`Roboto`

--- raw-align-default paged ---
// Text inside raw block should be unaffected by outer alignment by default.
#set align(center)
#set page(width: 180pt)
#set text(6pt)

```py
def something(x):
  return x

a = 342395823859823958329
b = 324923
```

--- raw-align-specified paged ---
// Text inside raw block should follow the specified alignment.
#set page(width: 180pt)
#set text(6pt)

#align(center, raw(
  lang: "typ",
  block: true,
  align: right,
  "#let f(x) = x\n#align(center, line(length: 1em))",
))

--- raw-align-invalid eval ---
// Error: 17-20 expected `start`, `left`, `center`, `right`, or `end`, found top
#set raw(align: top)

--- raw-inline-multiline paged ---
#set page(width: 180pt)
#set text(6pt)
#set raw(lang:"python")

Inline raws, multiline e.g. `for i in range(10):
  # Only this line is a comment.
  print(i)` or otherwise e.g. `print(j)`, are colored properly.

Inline raws, multiline e.g. `
# Appears blocky due to linebreaks at the boundary.
for i in range(10):
  print(i)
` or otherwise e.g. `print(j)`, are colored properly.

--- raw-highlight-typ paged ---
// Highlighting for Typst markup
#set page(width: auto)
```typ
#set heading(numbering: "1.")
= Chapter 1 <chap:1>
#lorem(100)

#let hi = "Hello World"
#show heading: emph
/ Chap: @chap:1[Chapter #hi]
- *Chap:* ch--ap
+ _*Chap:*_ ch~ap
1. _Chap:_ ch---ap
```

--- raw-highlight-typc paged ---
// Highlighting for Typst code
#set page(width: auto)
```typ
#set hello()
#set hello()
#set hello.world()
#set hello.my.world()
#let foo(x) = x * 2
#show heading: func
#show module.func: func
#show module.func: it => {}
#foo(ident: ident)
#hello
#hello()
#box[]
#hello.world
#hello.world()
#hello().world()
#hello.my.world
#hello.my.world()
#hello.my().world
#hello.my().world()
#{ hello }
#{ hello() }
#{ hello.world() }
#if foo []
```

--- raw-highlight-typm paged ---
// Highlighting for Typst math
#set page(width: auto)
```typm
1 + 2/3
sum_(i=1)^n i = (n(n+1))/2
binom(n, k) = n!/(k!(n - k)!)
2 / √(2pi) = sqrt(2) / √pi
3 * (1 - 2) <= #(3 * (1 + 2))
((a+b))/((c)^(d')_(e')_(f)'/(g)'/(h)!)
[\(a+b\)]/{\(c\)^[d']_{e'}_[|f|]'/[g]'/[\|h\|]!}
f_zeta(x), f_zeta(x)/1, f_zeta (x)
pi.alt + pi^arrow.l.long.double - π = ???
"string" - + * ::= & \
|=> & [|define(x-y_z: #1, x::= y; xyz; 0)|]
std.text(op("Red"), fill: red)
#std.text(math.op("Red"), fill: red)
```

--- raw-highlight-typm-idents paged ---
// Highlighting identifiers, field accesses and function calls in math
#set page(width: auto)
```typm
hello
hello-world
hello()
box[]
hello.world
hello.world()
hello-world()
hello_world()
hello.my.world()
emph(hello.my.world())
emph(hello.my().world)
emph(hello.my().world())
emph (hello.my().world())
#hello
#hello()
#hello.world
#hello.world()
#box[]
```

--- raw-highlight-typ-invalid-ints paged ---
// Invalid integers should still highlight as integers instead of black due to
// syntax errors.
// FUTURE: Invalid non-decimal numbers should highlight as integers.
#set page(width: auto)
```typ
#9223372036854775808
#92233720368547758089223372036854775808
#(-9223372036854775808)
#(-92233720368547758089223372036854775808)
#0x8000000000000000
#0xFFFFFFFFFFFFFFFFFFFFF
#0b0123456789
#0o0123456789
#0x9abc9xyz9XYZ9 // Only hex allows letters
#0o9abc9xyz9XYZ9
#0b9abc9xyz9XYZ9
#0xabcdéf // But only ASCII letters
#123abcdéf // Same for invalid suffixes
```

--- raw-highlight-rust paged ---
#set page(width: auto)

```rust
/// A state machine.
#[derive(Debug)]
enum State<'a> { A(u8), B(&'a str) }

fn advance(state: State<'_>) -> State<'_> {
    unimplemented!("state machine")
}
```

--- raw-highlight-py paged ---
#set page(width: auto)

```py
import this

def hi():
  print("Hi!")
```

--- raw-highlight-cpp paged ---
#set page(width: auto)

```cpp
#include <iostream>

int main() {
  std::cout << "Hello, world!";
}
```

--- raw-highlight-html paged ---
#set page(width: auto)

```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
  </head>
  <body>
    <h1>Topic</h1>
    <p>The Hypertext Markup Language.</p>
    <script>
      function foo(a, b) {
        return a + b + "string";
      }
    </script>
  </body>
</html>
```

--- raw-highlight-html-jinja2 paged ---
#set page(width: auto)
// FUTURE: Convert to raw syntax when we re-enable non-identifier language tags

#raw(block: true, lang: "html.j2", ```
<tbody>
  {% for row in data.rows %}
  <tr>
      {% for column in row %}
      <td>{{ column }}</td>
      {% endfor %}
  </tr>
  {% endfor %}
</tbody>
```.text)

--- raw-html html ---
This is ```typ *inline*```.
```typ
#[
  #set text(blue)
  *Hello* _world_!
]
```

--- raw-html-inline-spaces html ---
This has `double  spaces  inside`, which should be kept.

--- raw-line paged ---
#set page(width: 200pt)

```rs
fn main() {
    println!("Hello, world!");
}
```

#show raw.line: it => {
  box(stack(
    dir: ltr,
    box(width: 15pt)[#it.number],
    it.body,
  ))
  linebreak()
}

```rs
fn main() {
    println!("Hello, world!");
}
```

--- raw-line-alternating-fill paged ---
#set page(width: 200pt)
#show raw: it => stack(dir: ttb, ..it.lines)
#show raw.line: it => {
  box(
    width: 100%,
    height: 1.75em,
    inset: 0.25em,
    fill: if calc.rem(it.number, 2) == 0 {
      luma(90%)
    } else {
      white
    },
    align(horizon, stack(
      dir: ltr,
      box(width: 15pt)[#it.number],
      it.body,
    ))
  )
}

```typ
#show raw.line: block.with(
  fill: luma(60%)
);

Hello, world!

= A heading for good measure
```

--- raw-line-text-fill paged ---
#set page(width: 200pt)
#show raw.line: set text(fill: red)

```py
import numpy as np

def f(x):
    return x**2

x = np.linspace(0, 10, 100)
y = f(x)

print(x)
print(y)
```

--- raw-line-scripting paged empty ---

// Test line extraction works.

#show raw: code => {
  for i in code.lines {
    test(i.count, 10)
  }

  test(code.lines.at(0).text, "import numpy as np")
  test(code.lines.at(1).text, "")
  test(code.lines.at(2).text, "def f(x):")
  test(code.lines.at(3).text, "    return x**2")
  test(code.lines.at(4).text, "")
  test(code.lines.at(5).text, "x = np.linspace(0, 10, 100)")
  test(code.lines.at(6).text, "y = f(x)")
  test(code.lines.at(7).text, "")
  test(code.lines.at(8).text, "print(x)")
  test(code.lines.at(9).text, "print(y)")
  test(code.lines.at(10, default: none), none)
}

```py
import numpy as np

def f(x):
    return x**2

x = np.linspace(0, 10, 100)
y = f(x)

print(x)
print(y)
```

--- issue-3601-empty-raw paged ---
// Test that empty raw block with `typ` language doesn't cause a crash.
```typ
```

--- raw-empty-lines paged ---
// Test raw with multiple empty lines.

#show raw: block.with(width: 100%, fill: gray)

```




```

--- issue-3841-tabs-in-raw-type-code paged ---
// Tab chars were not rendered in raw blocks with lang: "typ(c)"
#raw("#if true {\n\tf()\t// typ\n}", lang: "typ")

#raw("if true {\n\tf()\t// typc\n}", lang: "typc")

```typ
#if true {
	// tabs around f()
	f()	// typ
}
```

```typc
if true {
	// tabs around f()
	f()	// typc
}
```

--- issue-6961-tab-crlf-raw-indent paged ---
#let snippet = (
  ```
  A
    BC
    D
  ```
)

#raw(
  snippet.text.replace("  ", "\t").replace("\n", "\r\n"),
  block: true,
)

--- issue-4662-math-mode-language-for-raw paged ---
// Test lang: "typm" syntax highlighting without enclosing dollar signs
#raw("pi^2", lang: "typm")

--- issue-2259-raw-color-overwrite paged ---
// Test that the color of a raw block is not overwritten
#show raw: set text(fill: blue)

`Hello, World!`

```rs
fn main() {
    println!("Hello, World!");
}
```

--- issue-3191-raw-justify paged ---
// Raw blocks should not be justified by default.
```
a b c --------------------
```

#show raw: set par(justify: true)
```
a b c --------------------
```

--- issue-3191-raw-normal-paragraphs-still-shrink paged ---
// In normal paragraphs, spaces should still be shrunk.
// The first line here serves as a reference, while the second
// uses non-breaking spaces to create an overflowing line
// (which should shrink).
~~~~No shrinking here

~~~~The~spaces~on~this~line~shrink

--- issue-3820-raw-space-when-end-with-backtick paged ---
```typ
`code`
```

  ```typ
  `code`
  ```

--- issue-5760-disable-cjk-latin-spacing-in-raw paged ---

```typ
#let hi = "你好world"
```

#show raw: set text(cjk-latin-spacing: auto)
```typ
#let hi = "你好world"
```

--- issue-6559-equality-between-raws eval ---

#test(`foo`, `foo`)
#assert.ne(`foo`, `bar`)

--- raw-theme-types paged ---
```typ
#let hi = "Hello World"
```

#set raw(theme: path("/assets/themes/halcyon.tmTheme"))
```typ
#let hi = "Hello World"
```

#set raw(theme: auto)
```typ
#let hi = "Hello World"
```

--- raw-theme-set-to-none paged ---
#set raw(theme: none)
```typ
#let foo = "bar"
```

--- raw-default-json-theme paged ---
```json
{
  "foo": "bar",
  "test": [
    "test",
    true,
    42,
    5.0,
    null
  ],
  "hi": {
    "this": "is a test!",
    "What is this?": "This is incredible text!"
  }
}
```

--- raw-default-yaml-theme paged ---
```yaml
foo: bar
test:
- test
- true
- 42
- 5
-
hi:
  this: is a test!
  What is this?: This is incredible text!
```

--- raw-unclosed eval ---
// Test unterminated raw text.
//
// Note: This test should be the final one in the file because it messes up
// syntax highlighting.
//
// Error: 1:1-2:1 unclosed raw text
`endless
//@@ file tests/suite/text_shift.typ
// Test sub- and superscript shifts.

--- sub-super paged ---
#let sq = box(square(size: 4pt))
#table(
  columns: 3,
  [Typo.], [Fallb.], [Synth.],
  [x#super[1#sq]], [x#super[5: #sq]], [x#super(typographic: false)[2 #sq]],
  [x#sub[1#sq]], [x#sub[5: #sq]], [x#sub(typographic: false)[2 #sq]],
)

--- sub-super-typographic paged ---
#set text(size: 20pt)
// Libertinus Serif supports "subs" and "sups" for `typo` and `sq`, but not for
// `synth`.
#let synth = [1,2,3]
#let typo = [123]
#let sq = [1#box(square(size: 4pt))2]
x#super(synth) x#super(typo) x#super(sq) \
x#sub(synth) x#sub(typo) x#sub(sq)

--- sub-super-italic-compensation paged ---
#set text(size: 20pt, style: "italic")
// Libertinus Serif supports "subs" and "sups" for `typo`, but not for `synth`.
#let synth = [1,2,3]
#let typo = [123]
#let sq = [1#box(square(size: 4pt))2]
x#super(synth) x#super(typo) x#super(sq) \
x#sub(synth) x#sub(typo) x#sub(sq)

--- sub-super-non-typographic paged ---
#set super(typographic: false, baseline: -0.25em, size: 0.7em)
n#super[1], n#sub[2], ... n#super[N]

--- super-underline paged ---
#set underline(stroke: 0.5pt, offset: 0.15em)
#set super(typographic: false)
#underline[A#super[4]] B \
A#super[#underline[4]] B \
A #underline(super[4]) B \
#set super(typographic: true)
#underline[A#super[4]] B \
A#super[#underline[4]] B \
A #underline(super[4]) B

--- super-highlight paged ---
#set super(typographic: false)
#highlight[A#super[4]] B \
A#super[#highlight[4]] B \
A#super(highlight[4]) \
#set super(typographic: true)
#highlight[A#super[4]] B \
A#super[#highlight[4]] B \
A#super(highlight[4])

--- super-1em paged empty ---
#set text(size: 10pt)
#super(context test(1em.to-absolute(), 10pt))

--- long-scripts paged ---
|longscript| \
|#super(typographic: true)[longscript]| \
|#super(typographic: false)[longscript]| \
|#sub(typographic: true)[longscript]| \
|#sub(typographic: false)[longscript]|

--- script-metrics-bundled-fonts paged ---
// Tests whether the script metrics are used properly by synthesizing
// superscripts and subscripts for all bundled fonts.

#set super(typographic: false)
#set sub(typographic: false)

#let test(font, weights, styles) = {
  for weight in weights {
    for style in styles {
      text(font: font, weight: weight, style: style)[Xx#super[Xx]#sub[Xx]]
      linebreak()
    }
  }
}

#test("DejaVu Sans Mono", ("regular", "bold"), ("normal", "oblique"))
#test("Libertinus Serif", ("regular", "semibold", "bold"), ("normal", "italic"))
#test("New Computer Modern", ("regular", "bold"), ("normal", "italic"))
#test("New Computer Modern Math", (400, 450, "bold"), ("normal",))

--- basic-sup-sub html ---
1#super[st], 2#super[nd], 3#super[rd].

log#sub[2], log#sub[3], log#sub[variable].

--- issue-7249-multiple-lookup-tables paged ---
// We increase the font size to make sure the difference is visible in the
// low-resolution reference image.
#set text(font: "Source Serif 4", size: 1.5em)
#set super(typographic: true)

A#super[test] \
A#super[test1] \
A#super[(test)] \
// Source Serif 4 does not support `sups` for backticks, so this should be
// synthesized.
A#super[test\`]
//@@ file tests/suite/text_smallcaps.typ
--- smallcaps paged ---
// Test smallcaps.
#smallcaps[Smallcaps]

--- smallcaps-show-rule paged ---
// There is no dedicated smallcaps font in typst-dev-assets, so we just use some
// other font to test this show rule.
#show smallcaps: set text(font: "PT Sans")
#smallcaps[Smallcaps]

#show smallcaps: set text(fill: red)
#smallcaps[Smallcaps]

--- smallcaps-all paged html ---
#smallcaps(all: false)[Test 012] \
#smallcaps(all: true)[Test 012]
//@@ file tests/suite/text_smartquote.typ
--- smartquote paged ---
#set text(lang: "en")
"The horse eats no cucumber salad" was the first sentence ever uttered on the 'telephone.'

--- smartquote-de paged ---
#set text(lang: "de")
"Das Pferd frisst keinen Gurkensalat" war der erste jemals am 'Fernsprecher' gesagte Satz.

--- smartquote-de-ch paged ---
#set text(lang: "de", region: "CH")
"Das Pferd frisst keinen Gurkensalat" war der erste jemals am 'Fernsprecher' gesagte Satz.

--- smartquote-es paged ---
#set text(lang: "es", region: none)
"El caballo no come ensalada de pepino" fue la primera frase pronunciada por 'teléfono'.

--- smartquote-es-mx paged ---
#set text(lang: "es", region: "MX")
"El caballo no come ensalada de pepino" fue la primera frase pronunciada por 'teléfono'.

--- smartquote-fr paged ---
#set text(lang: "fr")
"Le cheval ne mange pas de salade de concombres" est la première phrase jamais prononcée au 'téléphone'.

--- smartquote-fr-ch paged ---
#set text(lang: "fr", region: "CH")
"Le cheval ne mange pas de salade de concombres" est la première phrase jamais prononcée au 'téléphone'.

--- smartquote-fi paged ---
#set text(lang: "fi")
"Hevonen ei syö kurkkusalaattia" oli ensimmäinen koskaan 'puhelimessa' lausuttu lause.

--- smartquote-gl paged ---
#set text(lang: "gl")
"O cabalo non come ensalada de cogombro" foi a primeira frase pronunciada por 'teléfono'.

--- smartquote-el paged ---
#set text(lang: "el")
"Το άλογο δεν τρώει αγγουροσαλάτα" ήταν η πρώτη πρόταση που ειπώθηκε στο 'τηλέφωνο'.

--- smartquote-he paged ---
#set text(lang: "he")
"הסוס לא אוכל סלט מלפפונים" היה המשפט ההראשון שנאמר ב'טלפון'.

--- smartquote-ro paged ---
#set text(lang: "ro")
"Calul nu mănâncă salată de castraveți" a fost prima propoziție rostită vreodată la 'telefon'.

--- smartquote-ru paged ---
#set text(lang: "ru")
"Лошадь не ест салат из огурцов" - это была первая фраза, сказанная по 'телефону'.

--- smartquote-uk paged ---
#set text(lang: "uk")
"Кінь не їсть огірковий салат" — перше речення, коли-небудь вимовлене по 'телефону'.

--- smartquote-it paged ---
#set text(lang: "it")
"Il cavallo non mangia insalata di cetrioli" è stata la prima frase pronunciata al 'telefono'.

--- smartquote-la paged ---
#set text(lang: "la")
#set smartquote(alternative: true)
"Equus cucumeris sem non edit" prima sententia in 'telephono' prolata fuit.

--- smartquote-empty paged ---
// Test single pair of quotes.
""

--- smartquote-apostrophe paged ---
// Test sentences with numbers and apostrophes.
The 5'11" 'quick' brown fox jumps over the "lazy" dog's ear.

He said "I'm a big fella."

--- smartquote-escape paged ---
// Test escape sequences.
The 5\'11\" 'quick\' brown fox jumps over the \"lazy' dog\'s ear.

--- smartquote-slash paged ---
// Test that smartquotes can open before non-whitespace if not nested.
"Hello"/"World" \
'"Hello"/"World"' \
""Hello"/"World""

--- smartquote-close-before-letter paged ---
// Test that smartquotes can close before alphabetic letters.
Straight "A"s and "B"s

--- smartquote-prime paged ---
// Test that primes result after numbers when possible.
A 2" nail. \
'A 2" nail.' \
"A 2" nail."

--- smartquote-bracket paged ---
// Test that brackets indicate an opening quote.
"a ["b"] c" \
"a b"c"d e"

--- smartquote-disable paged ---
// Test turning smart quotes off.
He's told some books contain questionable "example text".

#set smartquote(enabled: false)
He's told some books contain questionable "example text".

--- smartquote-disabled-temporarily paged ---
// Test changing properties within text.
"She suddenly started speaking french: #text(lang: "fr", region: "CH")['Je suis une banane.']" Roman told me.

Some people's thought on this would be #[#set smartquote(enabled: false); "strange."]

--- smartquote-nesting paged ---
// Test nested double and single quotes.
"'test statement'" \
"'test' statement" \
"statement 'test'"

--- smartquote-nesting-twice paged html ---
When you said _that "he_ surely meant that 'she intended to say "I'm sorry"'", I was quite confused.

'#box[box]'

--- smartquote-inline-block html ---
Applies across #html.span["inline-level] elements".

Does not apply across #html.div["block-level] elements".

--- smartquote-with-embedding-chars paged ---
#set text(lang: "fr")
"#"\u{202A}"bonjour#"\u{202C}"" \
#"\u{202A}""bonjour"#"\u{202C}"

--- smartquote-custom paged ---
// Use language quotes for missing keys, allow partial reset
#set smartquote(quotes: "«»")
"Double and 'Single' Quotes"

#set smartquote(quotes: (double: auto, single: "«»"))
"Double and 'Single' Quotes"

--- smartquote-custom-complex paged ---
// Allow 2 graphemes
#set smartquote(quotes: "a\u{0301}a\u{0301}")
"Double and 'Single' Quotes"

#set smartquote(quotes: (single: "a\u{0301}a\u{0301}"))
"Double and 'Single' Quotes"

--- smartquote-custom-bad-string eval ---
// Error: 25-28 expected 2 characters, found 1 character
#set smartquote(quotes: "'")

--- smartquote-custom-bad-array eval ---
// Error: 25-35 expected 2 quotes, found 4 quotes
#set smartquote(quotes: ("'",) * 4)

--- smartquote-custom-bad-dict eval ---
// Error: 25-45 expected 2 quotes, found 4 quotes
#set smartquote(quotes: (single: ("'",) * 4))

--- issue-3662-pdf-smartquotes paged ---
// Smart quotes were not appearing in the PDF outline, because they didn't
// implement `PlainText`.
= It's "Unnormal Heading"
= It’s “Normal Heading”

#set smartquote(enabled: false)
= It's "Unnormal Heading"
= It's 'single quotes'
= It’s “Normal Heading”

--- issue-1041-smartquotes-in-outline paged ---
#set page(width: 15em)
#outline()

= "This" "is" "a" "test"

--- issue-1540-smartquotes-across-newlines paged ---
// Test that smart quotes are inferred correctly across newlines.
"test"#linebreak()"test"

"test"\
"test"

--- issue-5146-smartquotes-after-equations paged ---
$i$'s $i$ 's
//@@ file tests/suite/text_space.typ
// Test whitespace handling.

--- space-collapsing paged ---
// Spacing around code constructs.
A#let x = 1;B  #test(x, 1) \
C #let x = 2;D #test(x, 2) \
E#if true [F]G \
H #if true{"I"} J \
K #if true [L] else []M \
#let c = true; N#while c [#(c = false)O] P \
#let c = true; Q #while c { c = false; "R" } S \
T#for _ in (none,) {"U"}V
#let foo = "A" ; \
#foo;B \
#foo; B \
#foo ;B

--- space-collapsing-comments paged ---
// Test spacing with comments.
A/**/B/**/C \
A /**/ B/**/C \
A /**/B/**/ C

--- space-collapsing-with-h paged ---
// Test spacing collapsing before spacing.
#set align(right)
A #h(0pt) B #h(0pt) \
A B \
A #h(-1fr) B

--- text-font-just-a-space paged ---
// Test that a run consisting only of whitespace isn't trimmed.
A#text(font: "IBM Plex Serif")[ ]B

--- text-font-change-after-space paged ---
// Test font change after space.
Left #text(font: "IBM Plex Serif")[Right].

--- space-collapsing-linebreaks paged ---
// Test that linebreak consumed surrounding spaces.
#align(center)[A \ B \ C]

--- space-collapsing-stringy-linebreak paged ---
// Test that space at start of non-backslash-linebreak line isn't trimmed.
A#"\n" B

--- space-trailing-linebreak paged ---
// Test that trailing space does not force a line break.
LLLLLLLLLLLLLLLLLL R _L_

--- space-ideographic-kept paged ---
// Test that ideographic spaces are preserved.
#set text(lang: "ja", font: "Noto Serif CJK JP")

だろうか？　何のために！　私は、

--- space-thin-kept paged ---
// Test that thin spaces are preserved.
| | U+0020 regular space \
| | U+2009 thin space

--- space-eq-newline eval ---
// Test whether spaces with/without newlines compare equal.
#let parbreak = [

]
#let one-newline = [
]
#let no-newline = [ ]
// parbreak is not equal
#assert.ne(one-newline, parbreak)
// spaces are equal despite newlines
#assert.eq(one-newline, no-newline)

--- space-no-visible-properties eval ---
// Ensure that `space` has no visible properties.
#let space = [ ]
#test(space.fields(), (:))
#test(space.func()(), [ ])
// Error: 23-27 unexpected argument
#let _ = space.func()(true)

--- issue-792-newline-space-discarding paged ---
// Test whether spaces with/without newlines are discarded adjacent to
// Chinese/Japanese text.

// Discard spaces from just a newline/comment
换
行

注//
释

多行/*
     */注释

// Keep spaces from a space character
空 格

// With both a space and a newline it still discards
空格 //
注释

// Even if the spaces look like this
水
/**/ 果

--- newline-space-discarding-punctuation paged ---
#set page(width: auto)
// We collapse spaces next to any ideographic punctuation
你好，
你好。
“你好？”
你好。

// But not if the punctuation is ambiguous on both sides
“你好”
“你好”

--- newline-space-discarding-korean paged ---
// Korean doesn't collapse spaces on newlines
줄
바꿈

// Unless using ideographic punctuation
쉼표，
줄 바꿈

--- newline-space-discarding-strong paged ---
// Test newline space discarding with strong emphasis.
*空 *格

*换*
行

// This space still collapses because it is followed by a newline space.
*空格 *
换行

// The second space here also collapses because it follows a newline space.
空格
* 换行*

--- newline-space-discarding-regex-show-rule paged ---
// Test newline space discarding with regex show rules.
#show regex("^(new line|注 释|换行 newline|newline 换行)$"): set text(red)
#show regex("^(newline|注释|换行newline|newline换行)$"): set text(blue)

// Only Latin/Latin should keep the inferred space.
new
line

// Chinese/Chinese, Chinese/Latin, and Latin/Chinese all collapse the space.
注
释

换行
newline

newline
换行

--- newline-space-discarding-dynamic paged ---
// Test newline space discarding with dynamic variables.
#let foo = [水果] // collapses
#foo
#foo

#let foo = [fruit] // doesn't collapse
#foo
#foo

#let one-newline = [
]
#let no-newline = [ ]
啊#one-newline;啊 // collapses

啊#no-newline;啊 // doesn't collapse

--- newline-space-discarding-smart-quotes paged ---
// Test newline space discarding adjacent to smart quotes.
A "' A '" A

A
"'
A
'"
A

あ "' あ '" あ

// Only these spaces are discarded.
あ
"'
あ
'"
あ

--- newline-space-discarding-non-text paged ---
// Test newline space discarding adjacent to non-textual inline elements.
#let line = box(height: 1em, width: 1pt, fill: red)

A #line A

A
#line
A

あ #line あ

// Only these spaces are discarded.
あ
#line
あ
//@@ file tests/suite/visualize_block.typ
// SKIP

#let bg-rect(inner) = {
  rect(inset: 0pt, outset: 0pt, fill: luma(200), inner)
}

#let test-block(cap: "butt", dash: none, adjacent: none, radius: 0pt) = {
  let adjacent-stroke = if adjacent == none {
    none
  } else {
    (thickness: adjacent, paint: red, dash: dash)
  }
  bg-rect(
    block(
      height: 1.2cm,
      width: 1.5cm,
      stroke: (
        top: adjacent-stroke,
        right: (cap: cap, thickness: 6pt, dash: dash, paint: green.transparentize(40%)),
        bottom: (cap: cap, thickness: 2pt, dash: dash, paint: blue.transparentize(40%)),
        left: adjacent-stroke
      ),
      radius: radius,
    )
  )
}

#let another-block(cap: "butt", radius: 6pt, adjacent: none) = {
  let adjacent-stroke = if adjacent != none {
    adjacent
  } else {
    none
  }
  bg-rect(
    block(
      height: 1.2cm,
      width: 1.5cm,
      stroke: (
        top: adjacent-stroke,
        right: (cap: cap, thickness: 6pt),
        bottom: (cap: cap, thickness: 6pt),
        left: adjacent-stroke,
      ),
      radius: radius,
    )
  )
}
//@@ file tests/suite/visualize_circle.typ
// Test the `circle` function.

--- circle paged ---
// Default circle.
#stack(
  dir: ltr,
  spacing: 0.5em,
  circle(),
  circle[Hey]
)

--- circle-auto-sizing paged ---
// Test auto sizing.
#set circle(inset: 0pt)

Auto-sized circle.
#circle(fill: rgb("eb5278"), stroke: 2pt + black,
  align(center + horizon)[But, soft!]
)

Center-aligned rect in auto-sized circle.
#circle(fill: forest, stroke: conifer,
  align(center + horizon,
    rect(fill: conifer, inset: 5pt)[But, soft!]
  )
)

Rect in auto-sized circle.
#circle(fill: forest,
  rect(fill: conifer, stroke: white, inset: 4pt)[
    #set text(8pt)
    But, soft! what light through yonder window breaks?
  ]
)

Expanded by height.
#circle(stroke: black, align(center)[A \ B \ C])

--- circle-directly-in-rect paged ---
// Ensure circle directly in rect works.
#rect(width: 40pt, height: 30pt, fill: forest,
  circle(fill: conifer))

--- circle-relative-sizing paged ---
// Test relative sizing.
#set text(fill: white)
#show: rect.with(width: 100pt, height: 50pt, inset: 0pt, fill: rgb("aaa"))
#set align(center + horizon)
#stack(
  dir: ltr,
  spacing: 1fr,
  1fr,
  circle(radius: 10pt, fill: eastern, [A]),      // D=20pt
  circle(height: 60%, fill: eastern, [B]),       // D=30pt
  circle(width: 20% + 20pt, fill: eastern, [C]), // D=40pt
  1fr,
)

--- circle-radius-width-and-height eval ---
// Radius wins over width and height.
// Error: 23-34 unexpected argument: width
#circle(radius: 10pt, width: 50pt, height: 100pt, fill: eastern)

--- circle-sizing-options paged ---
// Test different ways of sizing.
#set page(width: 120pt, height: 40pt)
#stack(
  dir: ltr,
  spacing: 2pt,
  circle(radius: 5pt),
  circle(width: 10%),
  circle(height: 50%),
)

--- circle-size-beyond-default paged ---
// Test that setting a circle's height beyond its default sizes it correctly.
#circle()
#circle(height: 60pt)
#circle(width: 60pt)
#circle(radius: 30pt)

--- circle-beyond-page-width-overflows paged ---
// Test that sizing a circle beyond the page width correctly overflows the page.
#set page(height: 100pt)
#circle(width: 150%)
//@@ file tests/suite/visualize_color.typ
// Test color modification methods.

--- color-mix eval ---
// Compare both ways.
#test-repr(rgb(0%, 30.2%, 70.2%), rgb("004db3"))

// Alpha channel.
#test(rgb(255, 0, 0, 50%), rgb("ff000080"))

// Test color modification methods.
#test(rgb(25, 35, 45).lighten(10%), rgb(48, 57, 66))
#test(rgb(40, 30, 20).darken(10%), rgb(36, 27, 18))
#test(rgb("#133337").negate(space: rgb), rgb(236, 204, 200))
#test(white.lighten(100%), white)

// Color mixing, in Oklab space by default.
#test(rgb(color.mix(rgb("#ff0000"), rgb("#00ff00"))), rgb("#d0a800"))
#test(rgb(color.mix(rgb("#ff0000"), rgb("#00ff00"), space: oklab)), rgb("#d0a800"))
#test(rgb(color.mix(rgb("#ff0000"), rgb("#00ff00"), space: rgb)), rgb("#808000"))

#test(rgb(color.mix(red, green, blue)), rgb("#909282"))
#test(rgb(color.mix(red, blue, green)), rgb("#909282"))
#test(rgb(color.mix(blue, red, green)), rgb("#909282"))

// Mix with weights.
#test(rgb(color.mix((red, 50%), (green, 50%))), rgb("#c0983b"))
#test(rgb(color.mix((red, 0.5), (green, 0.5))), rgb("#c0983b"))
#test(rgb(color.mix((red, 5), (green, 5))), rgb("#c0983b"))
#test(rgb(color.mix((green, 5), (white, 0), (red, 5))), rgb("#c0983b"))
#test(color.mix((rgb("#aaff00"), 25%), (rgb("#aa00ff"), 75%), space: rgb), rgb("#aa40bf"))
#test(color.mix((rgb("#aaff00"), 50%), (rgb("#aa00ff"), 50%), space: rgb), rgb("#aa8080"))
#test(color.mix((rgb("#aaff00"), 75%), (rgb("#aa00ff"), 25%), space: rgb), rgb("#aabf40"))

// Mix in hue-based space.
#test(rgb(color.mix(red, blue, space: color.hsl)), rgb("#c408ff"))
#test(rgb(color.mix((red, 50%), (blue, 100%), space: color.hsl)), rgb("#5100f8"))
// Error: 6-51 cannot mix more than two colors in a hue-based space
#rgb(color.mix(red, blue, white, space: color.hsl))

--- color-conversion eval ---
// Test color conversion method kinds.
#test(rgb(rgb(10, 20, 30)).space(), rgb)
#test(color.linear-rgb(rgb(10, 20, 30)).space(), color.linear-rgb)
#test(oklab(rgb(10, 20, 30)).space(), oklab)
#test(oklch(rgb(10, 20, 30)).space(), oklch)
#test(color.hsl(rgb(10, 20, 30)).space(), color.hsl)
#test(color.hsv(rgb(10, 20, 30)).space(), color.hsv)
#test(cmyk(rgb(10, 20, 30)).space(), cmyk)
#test(luma(rgb(10, 20, 30)).space(), luma)

#test(rgb(color.linear-rgb(10, 20, 30)).space(), rgb)
#test(color.linear-rgb(color.linear-rgb(10, 20, 30)).space(), color.linear-rgb)
#test(oklab(color.linear-rgb(10, 20, 30)).space(), oklab)
#test(oklch(color.linear-rgb(10, 20, 30)).space(), oklch)
#test(color.hsl(color.linear-rgb(10, 20, 30)).space(), color.hsl)
#test(color.hsv(color.linear-rgb(10, 20, 30)).space(), color.hsv)
#test(cmyk(color.linear-rgb(10, 20, 30)).space(), cmyk)
#test(luma(color.linear-rgb(10, 20, 30)).space(), luma)

#test(rgb(oklab(10%, 20%, 30%)).space(), rgb)
#test(color.linear-rgb(oklab(10%, 20%, 30%)).space(), color.linear-rgb)
#test(oklab(oklab(10%, 20%, 30%)).space(), oklab)
#test(oklch(oklab(10%, 20%, 30%)).space(), oklch)
#test(color.hsl(oklab(10%, 20%, 30%)).space(), color.hsl)
#test(color.hsv(oklab(10%, 20%, 30%)).space(), color.hsv)
#test(cmyk(oklab(10%, 20%, 30%)).space(), cmyk)
#test(luma(oklab(10%, 20%, 30%)).space(), luma)

#test(rgb(oklch(60%, 40%, 0deg)).space(), rgb)
#test(color.linear-rgb(oklch(60%, 40%, 0deg)).space(), color.linear-rgb)
#test(oklab(oklch(60%, 40%, 0deg)).space(), oklab)
#test(oklch(oklch(60%, 40%, 0deg)).space(), oklch)
#test(color.hsl(oklch(60%, 40%, 0deg)).space(), color.hsl)
#test(color.hsv(oklch(60%, 40%, 0deg)).space(), color.hsv)
#test(cmyk(oklch(60%, 40%, 0deg)).space(), cmyk)
#test(luma(oklch(60%, 40%, 0deg)).space(), luma)

#test(rgb(color.hsl(10deg, 20%, 30%)).space(), rgb)
#test(color.linear-rgb(color.hsl(10deg, 20%, 30%)).space(), color.linear-rgb)
#test(oklab(color.hsl(10deg, 20%, 30%)).space(), oklab)
#test(oklch(color.hsl(10deg, 20%, 30%)).space(), oklch)
#test(color.hsl(color.hsl(10deg, 20%, 30%)).space(), color.hsl)
#test(color.hsv(color.hsl(10deg, 20%, 30%)).space(), color.hsv)
#test(cmyk(color.hsl(10deg, 20%, 30%)).space(), cmyk)
#test(luma(color.hsl(10deg, 20%, 30%)).space(), luma)

#test(rgb(color.hsv(10deg, 20%, 30%)).space(), rgb)
#test(color.linear-rgb(color.hsv(10deg, 20%, 30%)).space(), color.linear-rgb)
#test(oklab(color.hsv(10deg, 20%, 30%)).space(), oklab)
#test(oklch(color.hsv(10deg, 20%, 30%)).space(), oklch)
#test(color.hsl(color.hsv(10deg, 20%, 30%)).space(), color.hsl)
#test(color.hsv(color.hsv(10deg, 20%, 30%)).space(), color.hsv)
#test(cmyk(color.hsv(10deg, 20%, 30%)).space(), cmyk)
#test(luma(color.hsv(10deg, 20%, 30%)).space(), luma)

#test(rgb(cmyk(10%, 20%, 30%, 40%)).space(), rgb)
#test(color.linear-rgb(cmyk(10%, 20%, 30%, 40%)).space(), color.linear-rgb)
#test(oklab(cmyk(10%, 20%, 30%, 40%)).space(), oklab)
#test(oklch(cmyk(10%, 20%, 30%, 40%)).space(), oklch)
#test(color.hsl(cmyk(10%, 20%, 30%, 40%)).space(), color.hsl)
#test(color.hsv(cmyk(10%, 20%, 30%, 40%)).space(), color.hsv)
#test(cmyk(cmyk(10%, 20%, 30%, 40%)).space(), cmyk)
#test(luma(cmyk(10%, 20%, 30%, 40%)).space(), luma)

#test(rgb(luma(10%)).space(), rgb)
#test(color.linear-rgb(luma(10%)).space(), color.linear-rgb)
#test(oklab(luma(10%)).space(), oklab)
#test(oklch(luma(10%)).space(), oklch)
#test(color.hsl(luma(10%)).space(), color.hsl)
#test(color.hsv(luma(10%)).space(), color.hsv)
#test(cmyk(luma(10%)).space(), cmyk)
#test(luma(luma(10%)).space(), luma)

#test(rgb(1, 2, 3).to-hex(), "#010203")
#test(rgb(1, 2, 3, 4).to-hex(), "#01020304")
#test(luma(40).to-hex(), "#282828")
#test-repr(cmyk(4%, 5%, 6%, 7%).to-hex(), "#e2dcda")
#test-repr(rgb(cmyk(4%, 5%, 6%, 7%)), rgb(226, 220, 218))
#test-repr(rgb(luma(40%)), rgb(40%, 40%, 40%))
#test-repr(cmyk(luma(40)), cmyk(63.24%, 57.33%, 56.49%, 75.88%))
#test-repr(cmyk(rgb(1, 2, 3)), cmyk(66.67%, 33.33%, 0%, 98.82%))
#test-repr(luma(rgb(1, 2, 3)), luma(0.73%))
#test-repr(color.hsl(luma(40)), color.hsl(0deg, 0%, 15.69%))
#test-repr(color.hsv(luma(40)), color.hsv(0deg, 0%, 15.69%))
#test-repr(color.linear-rgb(luma(40)), color.linear-rgb(2.12%, 2.12%, 2.12%))
#test-repr(color.linear-rgb(rgb(1, 2, 3)), color.linear-rgb(0.03%, 0.06%, 0.09%))
#test-repr(color.hsl(rgb(1, 2, 3)), color.hsl(-150deg, 50%, 0.78%))
#test-repr(color.hsv(rgb(1, 2, 3)), color.hsv(-150deg, 66.67%, 1.18%))
#test-repr(oklab(luma(40)), oklab(27.68%, 0.0, 0.0, 100%))
#test-repr(oklab(rgb(1, 2, 3)), oklab(8.23%, -0.004, -0.007, 100%))
#test-repr(oklch(oklab(40%, 0.2, 0.2)), oklch(40%, 0.283, 45deg, 100%))
#test-repr(oklch(luma(40)), oklch(27.68%, 0.0, 72.49deg, 100%))
#test-repr(oklch(rgb(1, 2, 3)), oklch(8.23%, 0.008, 240.75deg, 100%))

--- color-spaces paged ---
// The different color spaces.
#let col = rgb(50%, 64%, 16%)
#box(square(size: 9pt, fill: col))
#box(square(size: 9pt, fill: rgb(col)))
#box(square(size: 9pt, fill: oklab(col)))
#box(square(size: 9pt, fill: oklch(col)))
#box(square(size: 9pt, fill: luma(col)))
#box(square(size: 9pt, fill: cmyk(col)))
#box(square(size: 9pt, fill: color.linear-rgb(col)))
#box(square(size: 9pt, fill: color.hsl(col)))
#box(square(size: 9pt, fill: color.hsv(col)))

--- color-space eval ---
// Test color kind method.
#test(rgb(1, 2, 3, 4).space(), rgb)
#test(cmyk(4%, 5%, 6%, 7%).space(), cmyk)
#test(luma(40).space(), luma)
#test(rgb(1, 2, 3, 4).space() != luma, true)

--- color-components eval ---
// Test color '.components()' without conversions.

#let test-components(col, ref, has-alpha: true) = {
  // Perform an approximate scalar comparison.
  let are-equal((a, b)) = {
    let to-float(x) = if type(x) == angle { x.rad() } else { float(x) }
    let epsilon = 1e-4 // The maximum error between both numbers
    test(type(a), type(b))
    calc.abs(to-float(a) - to-float(b)) < epsilon
  }

  let ref-without-alpha = if has-alpha { ref.slice(0, -1) } else { ref }
  test(col.components().len(), ref.len())
  assert(col.components().zip(ref).all(are-equal))
  assert(col.components(alpha: false).zip(ref-without-alpha).all(are-equal))
}
#test-components(rgb(1, 2, 3, 4), (0.39%, 0.78%, 1.18%, 1.57%))
#test-components(luma(40), (15.69%, 100%))
#test-components(luma(40, 50%), (15.69%, 50%))
#test-components(luma(40, 127), (15.69%, 49.8%))
#test-components(luma(40, 0), (15.69%, 0%))
#test-components(luma(40, 255), (15.69%, 100%))
#test-components(cmyk(4%, 5%, 6%, 7%), (4%, 5%, 6%, 7%), has-alpha: false)
#test-components(oklab(10%, 0.2, 0.4), (10%, 0.2, 0.4, 100%))
#test-components(oklch(10%, 0.2, 90deg), (10%, 0.2, 90deg, 100%))
#test-components(oklab(10%, 50%, 200%), (10%, 0.2, 0.8, 100%))
#test-components(oklch(10%, 50%, 90deg), (10%, 0.2, 90deg, 100%))
#test-components(color.linear-rgb(10%, 20%, 30%), (10%, 20%, 30%, 100%))
#test-components(color.hsv(10deg, 20%, 30%), (10deg, 20%, 30%, 100%))
#test-components(color.hsl(10deg, 20%, 30%), (10deg, 20%, 30%, 100%))

--- color-luma paged ---
// Test gray color conversion.
#stack(dir: ltr, rect(fill: luma(0)), rect(fill: luma(80%)))

--- color-rgb-out-of-range eval ---
// Error for values that are out of range.
// Error: 11-14 number must be between 0 and 255
#test(rgb(-30, 15, 50))

--- color-rgb-bad-string eval ---
// Error: 6-11 color string contains non-hexadecimal letters
#rgb("lol")

--- color-rgb-missing-argument-red eval ---
// Error: 2-7 missing argument: red component
#rgb()

--- color-rgb-missing-argument-blue eval ---
// Error: 2-11 missing argument: blue component
#rgb(0, 1)

--- color-rgb-bad-type eval ---
// Error: 21-26 expected integer or ratio, found boolean
#rgb(10%, 20%, 30%, false)

--- color-luma-empty-arguments eval ---
// Error: 7-13 missing argument: lightness component
// Hint: 7-13 try writing `white` or `luma(100%)`
#test(luma(), white)

--- color-luma-invalid-alpha eval ---
// Error: 7-16 expected integer or ratio, found string
#luma("invalid")

--- color-luma-invalid-lightness eval ---
// Error: 12-21 expected integer or ratio, found string
#luma(10%, "invalid")

--- color-luma-unexpected-argument eval ---
// Error: 12-22 unexpected argument: key
#luma(255, key: "val")

--- color-luma-unexpected-pos-argument eval ---
// Error: 17-22 unexpected argument
#luma(25%, 50%, "val")

--- color-rotate-bad-space eval ---
// Error: 27-30 expected `oklch`, `color.hsl`, or `color.hsv`
#red.rotate(90deg, space: rgb)

--- color-mix-bad-amount-type eval ---
// Error: 12-24 expected float or ratio, found string
// Error: 26-39 expected float or ratio, found string
#color.mix((red, "yes"), (green, "no"), (green, 10%))

--- color-mix-bad-value eval ---
// Error: 12-23 expected a color or color-weight pair
#color.mix((red, 1, 2))

--- color-mix-bad-space-type eval ---
// Error: 31-38 expected spot colorant, function, or auto, found string
#color.mix(red, green, space: "cyber")

--- color-mix-bad-space-value-1 eval ---
// Error: 31-36 expected `rgb`, `luma`, `cmyk`, `oklab`, `oklch`, `color.linear-rgb`, `color.hsl`, `color.hsv`, or spot colorant
#color.mix(red, green, space: image)

--- color-mix-bad-space-value-2 eval ---
// Error: 31-41 expected `rgb`, `luma`, `cmyk`, `oklab`, `oklch`, `color.linear-rgb`, `color.hsl`, `color.hsv`, or spot colorant
#color.mix(red, green, space: calc.round)

--- color-cmyk-ops paged ---
// Test CMYK color conversion.
#let c = cmyk(50%, 64%, 16%, 17%)
#stack(
  dir: ltr,
  spacing: 1fr,
  rect(width: 1cm, fill: cmyk(69%, 11%, 69%, 41%)),
  rect(width: 1cm, fill: c),
  rect(width: 1cm, fill: c.negate(space: cmyk)),
)

#for x in range(0, 11) {
  box(square(size: 9pt, fill: c.lighten(x * 10%)))
}
#for x in range(0, 11) {
  box(square(size: 9pt, fill: c.darken(x * 10%)))
}

--- color-outside-srgb-gamut paged ---
// Colors outside the sRGB gamut.
#box(square(size: 9pt, fill: oklab(90%, -0.2, -0.1)))
#box(square(size: 9pt, fill: oklch(50%, 0.5, 0deg)))

--- color-rotate-hue paged ---
// Test hue rotation.
#let col = rgb(50%, 64%, 16%)

// Oklch
#for x in range(0, 11) {
  box(square(size: 9pt, fill: rgb(col).rotate(x * 36deg)))
}

// HSL
#for x in range(0, 11) {
  box(square(size: 9pt, fill: rgb(col).rotate(x * 36deg, space: color.hsl)))
}

// HSV
#for x in range(0, 11) {
  box(square(size: 9pt, fill: rgb(col).rotate(x * 36deg, space: color.hsv)))
}

--- color-saturation paged ---
// Test saturation.
#let col = color.hsl(180deg, 0%, 50%)
#for x in range(0, 11) {
  box(square(size: 9pt, fill: col.saturate(x * 10%)))
}

#let col = color.hsl(180deg, 100%, 50%)
#for x in range(0, 11) {
  box(square(size: 9pt, fill: col.desaturate(x * 10%)))
}

#let col = color.hsv(180deg, 0%, 50%)
#for x in range(0, 11) {
  box(square(size: 9pt, fill: col.saturate(x * 10%)))
}

#let col = color.hsv(180deg, 100%, 50%)
#for x in range(0, 11) {
  box(square(size: 9pt, fill: col.desaturate(x * 10%)))
}

--- color-luma-ops eval ---
// Test gray color modification.
#test-repr(luma(20%).lighten(50%), luma(60%))
#test-repr(luma(80%).darken(20%), luma(64%))
#test-repr(luma(80%).negate(space: luma), luma(20%))

--- color-transparentize eval ---
// Test alpha modification.
#test-repr(luma(100%, 100%).transparentize(50%), luma(100%, 50%))
#test-repr(luma(100%, 100%).transparentize(75%), luma(100%, 25%))
#test-repr(luma(100%, 50%).transparentize(50%), luma(100%, 25%))
#test-repr(luma(100%, 10%).transparentize(250%), luma(100%, 0%))
#test-repr(luma(100%, 40%).transparentize(-50%), luma(100%, 70%))
#test-repr(luma(100%, 0%).transparentize(-100%), luma(100%, 100%))

--- color-opacify eval ---
#test-repr(luma(100%, 50%).opacify(50%), luma(100%, 75%))
#test-repr(luma(100%, 20%).opacify(100%), luma(100%, 100%))
#test-repr(luma(100%, 100%).opacify(250%), luma(100%, 100%))
#test-repr(luma(100%, 50%).opacify(-50%), luma(100%, 25%))
#test-repr(luma(100%, 0%).opacify(0%), luma(100%, 0%))

--- issue-color-mix-luma paged ---
// When mixing luma colors, we accidentally used the wrong component.
#rect(fill: gradient.linear(black, silver, space: luma))

--- issue-4361-transparency-leak paged ---
// Ensure that transparency doesn't leak from shapes to images in PDF. The PNG
// test doesn't validate it, but at least we can discover regressions on the PDF
// output with a PDF comparison script.
#rect(fill: red.transparentize(50%))
#image("/assets/images/tiger.jpg", width: 45pt)

--- spot-color-basic paged ---
// Test basic spot color creation and rendering.
#let pantone = color.spot("PANTONE 2221 C", eastern)
#let tinted = pantone.tint(80%)
#box(square(size: 20pt, fill: tinted))

--- spot-color-tints paged ---
// Test different tint levels of the same spot colorant.
#let pantone = color.spot("PANTONE 185 C", rgb(89.4%, 0.7%, 17%))
#box(square(size: 15pt, fill: pantone.tint(100%)))
#box(square(size: 15pt, fill: pantone.tint(75%)))
#box(square(size: 15pt, fill: pantone.tint(50%)))
#box(square(size: 15pt, fill: pantone.tint(25%)))
#box(square(size: 15pt, fill: pantone.tint(0%)))

--- spot-color-none paged ---
// Test spot color with name set to none.
#let varnish = color.spot(none, luma(0%))
#let layer = varnish.tint(100%)
#box(square(size: 20pt, fill: layer))

--- spot-color-components eval ---
// Test spot color components method.
#let pantone = color.spot("PANTONE 2221 C", eastern)
#let tinted = pantone.tint(80%)
#test(tinted.components(), (80%,))
#test(tinted.components(alpha: false), (80%,))

--- spot-color-tint-out-of-range-negative eval ---
#let pantone = color.spot("PANTONE 2221 C", eastern)
// Error: 15-19 spot color tint must be positive
#pantone.tint(-10%)

--- spot-color-tint-out-of-range-large eval ---
#let pantone = color.spot("PANTONE 2221 C", eastern)
// Error: 15-19 spot color tint must not exceed 100%
#pantone.tint(120%)

--- spot-color-lighten paged ---
// Test lighten on spot colors.
#let pantone = color.spot("PANTONE 185 C", rgb(89.4%, 0.7%, 17%))
#let base = pantone.tint(80%)
#let light = base.lighten(25%)
#box(square(size: 15pt, fill: base))
#box(square(size: 15pt, fill: light))

--- spot-color-darken paged ---
// Test darken on spot colors.
#let pantone = color.spot("PANTONE 185 C", rgb(89.4%, 0.7%, 17%))
#let base = pantone.tint(50%)
#let dark = base.darken(25%)
#box(square(size: 15pt, fill: base))
#box(square(size: 15pt, fill: dark))

--- spot-color-negate eval ---
// Test negate on spot colors.
#let pantone = color.spot("PANTONE 185 C", rgb(89.4%, 0.7%, 17%))
#let base = pantone.tint(70%)
#let neg = base.negate()
#let eps = 0.0001 * 100%
#let components = neg.components()
#if components.len() != 1 { panic("expected 1 component, got " + components.len()) }
#if calc.abs(components.at(0) - 30%) > eps { panic("spot color negation failed") }

--- spot-colorant-none paged ---
#let c = color.spot(none, red)
#box(square(size: 15pt, fill: c.tint(50%)))

--- spot-colorant-all paged ---
#let c = color.spot("all", blue)
#box(square(size: 15pt, fill: c.tint(75%)))

--- spot-color-as-fallback eval ---
#let pantone = color.spot("PANTONE 2221 C", eastern)
// Error: 30-47 spot color found where process color is required
#color.spot("PANTONE 185 C", pantone.tint(50%))

--- spot-colorant-as-fallback eval ---
#let pantone = color.spot("PANTONE 2221 C", eastern)
// Error: 30-37 expected color, found spot colorant
#color.spot("PANTONE 185 C", pantone)

--- spot-colorant-inconsistent-fallback pdf ---
#set pdf(standard: "a-2a")
// Error: spot colorant `"PANTONE 185 C"` appeared with multiple distinct fallback colors
// Hint: define a particular spot colorant once and store it in a variable to reuse it later
#set document(date: datetime(year: 1970, month: 1, day: 1))
#rect(fill: color.spot("PANTONE 185 C", teal).tint(100%))
#rect(fill: color.spot("PANTONE 185 C", yellow).tint(100%))
//@@ file tests/suite/visualize_curve.typ
// Test curves.

--- curve-move-single paged ---
#curve(
  stroke: 5pt,
  curve.move((0pt,  30pt)),
  curve.line((30pt, 30pt)),
  curve.line((15pt, 0pt)),
  curve.close()
)

--- curve-move-multiple-even-odd paged ---
#curve(
  fill: yellow,
  stroke: yellow.darken(20%),
  fill-rule: "even-odd",
  curve.move((10pt, 10pt)),
  curve.line((20pt, 10pt)),
  curve.line((20pt, 20pt)),
  curve.close(),
  curve.move((0pt, 5pt)),
  curve.line((25pt, 5pt)),
  curve.line((25pt, 30pt)),
  curve.close(mode: "smooth"),
)

--- curve-move-multiple-non-zero paged ---
#curve(
  fill: yellow,
  stroke: yellow.darken(20%),
  curve.move((10pt, 10pt)),
  curve.line((20pt, 10pt)),
  curve.line((20pt, 20pt)),
  curve.close(),
  curve.move((0pt, 5pt)),
  curve.line((25pt, 5pt)),
  curve.line((25pt, 30pt)),
  curve.close(mode: "smooth"),
)

--- curve-multiple-non-closed paged ---
#curve(
  stroke: 2pt,
  curve.line((20pt, 0pt)),
  curve.move((0pt,  10pt)),
  curve.line((20pt, 10pt)),
  curve.move((0pt,  20pt)),
  curve.line((20pt, 20pt)),
)

--- curve-line paged ---
#curve(
  fill: purple,
  stroke: 3pt + purple.lighten(50%),
  curve.move((0pt, 0pt)),
  curve.line((30pt, 30pt)),
  curve.line((0pt, 30pt)),
  curve.line((30pt, 0pt)),
)

--- curve-quad-mirror paged ---
#curve(
  stroke: 2pt,
  curve.quad((20pt, 40pt), (40pt, 40pt), relative: true),
  curve.quad(auto, (40pt, -40pt), relative: true),
)

--- curve-cubic-mirror paged ---
#set page(height: 100pt)
#curve(
  fill: red,
  curve.move((0%, 0%)),
  curve.cubic((-4%, 4%), (54%, 46%), (50%, 50%)),
  curve.cubic(auto, (4%, 54%), (0%, 50%)),
  curve.cubic(auto, (54%, 4%), (50%, 0%)),
  curve.close(),
)

--- curve-cubic-inflection paged ---
#set page(height: 120pt)
#curve(
  fill: blue.lighten(80%),
  stroke: blue,
  curve.move((30%, 0%)),
  curve.cubic((10%, 0%), (10%, 60%), (30%, 60%)),
  curve.cubic(none, (110%, 0%), (50%, 30%)),
  curve.cubic((110%, 30%), (65%, 30%), (30%, 0%)),
  curve.close(mode: "straight")
)

--- curve-close-smooth paged ---
#curve(
  fill: blue.lighten(80%),
  stroke: blue,
  curve.move((0pt, 40pt)),
  curve.cubic((0pt, 70pt), (10pt, 80pt), (40pt, 80pt)),
  curve.cubic(auto, (80pt, 70pt), (80pt, 40pt)),
  curve.cubic(auto, (70pt, 0pt), (40pt, 0pt)),
  curve.close(mode: "smooth")
)

--- curve-close-straight paged ---
#curve(
  fill: blue.lighten(80%),
  stroke: blue,
  curve.move((0pt, 40pt)),
  curve.cubic((0pt, 70pt), (10pt, 80pt), (40pt, 80pt)),
  curve.cubic(auto, (80pt, 70pt), (80pt, 40pt)),
  curve.cubic(auto, (70pt, 0pt), (40pt, 0pt)),
  curve.close(mode: "straight")
)

--- curve-close-intersection paged ---
#curve(
  fill: yellow,
  stroke: black,
  curve.move((10pt, 10pt)),
  curve.cubic((5pt, 20pt), (15pt, 20pt), (20pt, 0pt), relative: true),
  curve.cubic(auto, (15pt, -10pt), (20pt, 0pt), relative: true),
  curve.close(mode: "straight")
)

--- curve-stroke-gradient paged ---
#set page(width: auto)
#let down = curve.line((40pt, 40pt), relative: true)
#let up = curve.line((40pt, -40pt), relative: true)

#curve(
  stroke: 4pt + gradient.linear(red, blue),
  down, up, down, up, down,
)

--- curve-stroke-gradient-sharp paged ---
#set page(width: auto)
#let down = curve.line((40pt, 40pt), relative: true)
#let up = curve.line((40pt, -40pt), relative: true)

#curve(
  stroke: 4pt + gradient.linear(red, blue).sharp(3),
  down, up, down, up, down,
)

--- curve-fill-rule paged ---
#stack(
  dir: ltr,
  curve(
  fill: red,
  fill-rule: "non-zero",
  curve.move((25pt, 0pt)),
  curve.line((10pt, 50pt)),
  curve.line((50pt, 20pt)),
  curve.line((0pt, 20pt)),
  curve.line((40pt, 50pt)),
  curve.close()
  ),
  curve(
    fill: red,
    fill-rule: "even-odd",
    curve.move((25pt, 0pt)),
    curve.line((10pt, 50pt)),
    curve.line((50pt, 20pt)),
    curve.line((0pt, 20pt)),
    curve.line((40pt, 50pt)),
    curve.close()
  )
)

--- curve-infinite-length paged ---
// Error: 2-67 cannot create curve with infinite size
#curve(curve.move((0pt, 0pt)), curve.line((float.inf * 1pt, 0pt)))

--- issue-curve-in-sized-container paged ---
// Curves/Paths used to implement `LayoutMultiple` rather than `LayoutSingle`
// without fulfilling the necessary contract of respecting region expansion.
#block(
  fill: aqua,
  width: 20pt,
  height: 15pt,
  curve(
    curve.move((0pt, 0pt)),
    curve.line((10pt, 10pt)),
  ),
)
//@@ file tests/suite/visualize_ellipse.typ
// Test the `ellipse` function.

--- ellipse paged ---
// Default ellipse.
#ellipse()

--- ellipse-auto-sizing paged ---
#set rect(inset: 0pt)
#set ellipse(inset: 0pt)

Rect in ellipse in fixed rect.
#rect(width: 3cm, height: 2cm, fill: rgb("2a631a"),
  ellipse(fill: forest, width: 100%, height: 100%,
    rect(fill: conifer, width: 100%, height: 100%,
      align(center + horizon)[
        Stuff inside an ellipse!
      ]
    )
  )
)

Auto-sized ellipse.
#ellipse(fill: conifer, stroke: 3pt + forest, inset: 3pt)[
  #set text(8pt)
  But, soft! what light through yonder window breaks?
]


An inline
#box(ellipse(width: 8pt, height: 6pt, outset: (top: 3pt, rest: 5.5pt)))
ellipse.
//@@ file tests/suite/visualize_gradient.typ
--- gradient-linear-angled paged ---
// Test gradients with direction.
#set page(width: 90pt)
#grid(
  gutter: 3pt,
  columns: 4,
  ..range(0, 360, step: 15).map(i => box(
    height: 15pt,
    width: 15pt,
    fill: gradient.linear(angle: i * 1deg, (red, 0%), (blue, 100%)),
  ))
)

--- gradient-linear-spaces paged ---
// CMYK is excluded here and tested separately below because it's currently not
// 100% reproducible for SVG with the colors used in this test. (The color value
// is off by one on macOS. This might be due to different SIMD instruction sets
// being used by CMYK conversion in moxcms. We should investigate whether we can
// somehow fix this in the future.)
#set page(height: auto, margin: 0pt)
#set block(spacing: 0pt)
#let spaces = (
  ("HSV", color.hsv),
  ("HSL", color.hsl),
  ("Oklch", color.oklch),
  ("Oklab", color.oklab),
  ("sRGB", color.rgb),
  ("linear-RGB", color.linear-rgb),
  ("Luma", color.luma),
)
#for (name, space) in spaces {
  block(
    width: 100%,
    inset: 4pt,
    fill: gradient.linear(yellow, blue, space: space),
    name,
  )
}

--- gradient-linear-cmyk paged ---
// Test that CMYK works on gradients
#set page(margin: 0pt, width: 100pt, height: auto)

#let violet = cmyk(75%, 80%, 0%, 0%)
#let blue = cmyk(75%, 30%, 0%, 0%)

#rect(
  width: 100%,
  height: 10pt,
  fill: gradient.linear(violet, blue)
)

#rect(
  width: 100%,
  height: 10pt,
  fill: gradient.linear(rgb(violet), rgb(blue))
)

// In PDF format, this gradient can look different from the others.
// This is because PDF readers do weird things with CMYK.
#rect(
  width: 100%,
  height: 10pt,
  fill: gradient.linear(violet, blue, space: cmyk)
)

--- gradient-linear-relative-parent paged ---
// The image should look as if there is a single gradient that is being used for
// both the page and the rectangles.
#let grad = gradient.linear(red, blue, green, purple, relative: "parent")
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#place(top + right, my-rect)
#place(bottom + center, rotate(45deg, my-rect))

--- gradient-linear-relative-self paged ---
// The image should look as if there are multiple gradients, one for each
// rectangle.
#let grad = gradient.linear(red, blue, green, purple, relative: "self")
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#place(top + right, my-rect)
#place(bottom + center, rotate(45deg, my-rect))

--- gradient-linear-relative-parent-block paged ---
// The image should look as if there are two nested gradients, one for the page
// and one for a nested block. The rotated rectangles are not visible because
// they are relative to the block.
#let grad = gradient.linear(red, blue, green, purple, relative: "parent")
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#block(
  width: 40pt,
  height: 40pt,
  inset: 2.5pt,
  fill: grad,
)[
  #place(top + right, my-rect)
  #place(bottom + center, rotate(45deg, my-rect))
]

--- gradient-linear-repeat-and-mirror-1 paged ---
// Test repeated gradients.
#rect(
  height: 40pt,
  width: 100%,
  fill: gradient.linear(..color.map.inferno).repeat(2, mirror: true)
)

--- gradient-linear-repeat-and-mirror-2 paged ---
#rect(
  height: 40pt,
  width: 100%,
  fill: gradient.linear(..color.map.rainbow).repeat(2, mirror: true),
)

--- gradient-linear-repeat-and-mirror-3 paged ---
#rect(
  height: 40pt,
  width: 100%,
  fill: gradient.linear(..color.map.rainbow).repeat(5, mirror: true)
)

--- gradient-linear-sharp-and-repeat paged ---
#rect(
  height: 40pt,
  width: 100%,
  fill: gradient.linear(..color.map.rainbow).sharp(10).repeat(5, mirror: false)
)

--- gradient-linear-sharp-repeat-and-mirror paged ---
#rect(
  height: 40pt,
  width: 100%,
  fill: gradient.linear(..color.map.rainbow).sharp(10).repeat(5, mirror: true)
)

--- gradient-linear-sharp paged ---
#square(
  size: 100pt,
  fill: gradient.linear(..color.map.rainbow, space: color.hsl).sharp(10),
)
#square(
  size: 100pt,
  fill: gradient.radial(..color.map.rainbow, space: color.hsl).sharp(10),
)
#square(
  size: 100pt,
  fill: gradient.conic(..color.map.rainbow, space: color.hsl).sharp(10),
)

--- gradient-linear-sharp-and-smooth paged ---
#square(
  size: 100pt,
  fill: gradient.linear(..color.map.rainbow, space: color.hsl).sharp(10, smoothness: 40%),
)
#square(
  size: 100pt,
  fill: gradient.radial(..color.map.rainbow, space: color.hsl).sharp(10, smoothness: 40%),
)
#square(
  size: 100pt,
  fill: gradient.conic(..color.map.rainbow, space: color.hsl).sharp(10, smoothness: 40%),
)

--- gradient-linear-stroke paged ---
#align(center + top, square(size: 50pt, fill: black, stroke: 5pt + gradient.linear(red, blue)))

--- gradient-fill-and-stroke paged ---
#set page(width: auto)
#let stroke = 10pt + gradient.linear(green, yellow, blue).sharp(5)
#let fill = gradient.linear(lime, yellow.lighten(60%), aqua).sharp(5)
#let scale = gradient.linear(black, white, black, white, black).sharp(5)
#let marked = (shape) => {
  shape
  place(center + top, line(length: 60pt, stroke: scale))
  place(center + horizon, line(length: 50pt, stroke: scale))
}
#grid(columns: 2, gutter: 15pt,
  marked(rect(width: 50pt, height: 50pt, radius: 0pt, stroke: stroke, fill: fill)),
  marked(rect(width: 50pt, height: 50pt, radius: 20pt, stroke: stroke, fill: fill)),
  marked(curve(stroke: stroke, fill: fill, curve.line((50pt, 0pt)), curve.line((50pt, 50pt)), curve.line((0pt, 50pt)), curve.close())),
  marked(circle(radius: 25pt, stroke: stroke, fill: fill)),
)

--- gradient-linear-stroke-relative-parent paged ---
// The image should look as if there is a single gradient that is being used for
// both the circle stroke and the block fill.
#align(
  center + horizon,
  block(
    width: 50pt,
    height: 50pt,
    fill: gradient.linear(red, blue).sharp(4),
    circle(
      radius: 18pt,
      stroke: 5pt + gradient.linear(red, blue, relative: "parent").sharp(4),
    )
  )
)

--- gradient-linear-line paged ---
// Test gradient on lines
#set page(width: 100pt, height: 100pt)
#line(length: 100%, stroke: 1pt + gradient.linear(red, blue))
#line(length: 100%, angle: 10deg, stroke: 1pt + gradient.linear(red, blue))
#line(length: 100%, angle: 10deg, stroke: 1pt + gradient.linear(red, blue, relative: "parent"))

--- gradient-radial-hsl paged ---
#square(
  size: 100pt,
  fill: gradient.radial(..color.map.rainbow, space: color.hsl),
)

--- gradient-radial-center paged ---
#grid(
  columns: 2,
  square(
    size: 50pt,
    fill: gradient.radial(..color.map.rainbow, space: color.hsl, center: (0%, 0%)),
  ),
  square(
    size: 50pt,
    fill: gradient.radial(..color.map.rainbow, space: color.hsl, center: (0%, 100%)),
  ),
  square(
    size: 50pt,
    fill: gradient.radial(..color.map.rainbow, space: color.hsl, center: (100%, 0%)),
  ),
  square(
    size: 50pt,
    fill: gradient.radial(..color.map.rainbow, space: color.hsl, center: (100%, 100%)),
  ),
)

--- gradient-radial-radius paged ---
#square(
  size: 50pt,
  fill: gradient.radial(..color.map.rainbow, space: color.hsl, radius: 10%),
)
#square(
  size: 50pt,
  fill: gradient.radial(..color.map.rainbow, space: color.hsl, radius: 72%),
)

--- gradient-radial-focal-center-and-radius paged ---
#circle(
  radius: 25pt,
  fill: gradient.radial(white, rgb("#8fbc8f"), focal-center: (35%, 35%), focal-radius: 5%),
)
#circle(
  radius: 25pt,
  fill: gradient.radial(white, rgb("#8fbc8f"), focal-center: (75%, 35%), focal-radius: 5%),
)

--- gradient-radial-relative-parent paged ---
// The image should look as if there is a single gradient that is being used for
// both the page and the rectangles.
#let grad = gradient.radial(red, blue, green, purple, relative: "parent");
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#place(top + right, my-rect)
#place(bottom + center, rotate(45deg, my-rect))

--- gradient-radial-relative-self paged ---
// The image should look as if there are multiple gradients, one for each
// rectangle.
#let grad = gradient.radial(red, blue, green, purple, relative: "self");
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#place(top + right, my-rect)
#place(bottom + center, rotate(45deg, my-rect))

--- gradient-radial-text paged ---
// Test that gradient fills on text.
// The solid bar gradients are used to make sure that all transforms are
// correct: if you can see the text through the bar, then the gradient is
// misaligned to its reference container.
#set page(width: 200pt, height: auto, margin: 10pt)
#set par(justify: true)
#set text(fill: gradient.radial(red, blue))
#lorem(30)

--- gradient-conic paged ---
#square(
  size: 50pt,
  fill: gradient.conic(..color.map.rainbow, space: color.hsv),
)

--- gradient-conic-center-shifted-1 paged ---
#square(
  size: 50pt,
  fill: gradient.conic(..color.map.rainbow, space: color.hsv, center: (10%, 10%)),
)

--- gradient-conic-center-shifted-2 paged ---
#square(
  size: 50pt,
  fill: gradient.conic(..color.map.rainbow, space: color.hsv, center: (90%, 90%)),
)

--- gradient-conic-angled paged ---
#square(
  size: 50pt,
  fill: gradient.conic(..color.map.rainbow, space: color.hsv, angle: 90deg),
)

--- gradient-linear-angled-aspect-ratio paged ---
#let grad = gradient.linear(angle: 135deg, ..color.map.inferno).sharp(6)
#grid(
  columns: 2,
  gutter: 5pt,
  rect(width: 70pt, height: 70pt, fill: grad),
  rect(width: 25pt, height: 70pt, fill: grad),
  rect(width: 70pt, height: 25pt, fill: grad),
)

--- gradient-conic-angled-aspect-ratio paged ---
#let grad = gradient.conic(center: (70%, 30%), angle: 135deg, ..color.map.inferno)
#grid(
  columns: 2,
  gutter: 5pt,
  rect(width: 70pt, height: 70pt, fill: grad),
  rect(width: 25pt, height: 70pt, fill: grad),
  rect(width: 70pt, height: 25pt, fill: grad),
)

--- gradient-radial-aspect-ratio paged ---
#let grad = gradient.radial(
  center: (70%, 30%),
  focal-center: (50%, 50%),
  focal-radius: 10%,
  ..color.map.inferno
).sharp(5)
#grid(
  columns: 2,
  gutter: 5pt,
  rect(width: 70pt, height: 70pt, fill: grad),
  rect(width: 25pt, height: 70pt, fill: grad),
  rect(width: 70pt, height: 25pt, fill: grad),
)

--- gradient-conic-oklab paged ---
// Test in Oklab space for reference.
#set page(
  width: 100pt,
  height: 100pt,
  fill: gradient.conic(red, purple, space: oklab)
)

--- gradient-conic-oklch paged ---
// Test in OkLCH space.
#set page(
  width: 100pt,
  height: 100pt,
  fill: gradient.conic(red, purple, space: oklch)
)

--- gradient-conic-hsv paged ---
// Test in HSV space.
#set page(
  width: 100pt,
  height: 100pt,
  fill: gradient.conic(red, purple, space: color.hsv)
)

--- gradient-conic-hsl paged ---
// Test in HSL space.
#set page(
  width: 100pt,
  height: 100pt,
  fill: gradient.conic(red, purple, space: color.hsl)
)

--- gradient-conic-relative-parent paged ---
// The image should look as if there is a single gradient that is being used for
// both the page and the rectangles.
#let grad = gradient.conic(red, blue, green, purple, relative: "parent");
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#place(top + right, my-rect)
#place(bottom + center, rotate(45deg, my-rect))

--- gradient-conic-relative-self paged ---
// The image should look as if there are multiple gradients, one for each
// rectangle.
#let grad = gradient.conic(red, blue, green, purple, relative: "self");
#let my-rect = rect(width: 50%, height: 50%, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
  fill: grad,
  background: place(top + left, my-rect),
)
#place(top + right, my-rect)
#place(bottom + center, rotate(45deg, my-rect))

--- gradient-conic-stroke paged ---
#align(
  center + bottom,
  square(
    size: 50pt,
    fill: black,
    stroke: 10pt + gradient.conic(red, blue)
  )
)

--- gradient-conic-text paged ---
#set page(width: 200pt, height: auto, margin: 10pt)
#set par(justify: true)
#set text(fill: gradient.conic(red, blue, angle: 45deg))
#lorem(30)

--- gradient-text-bad-relative eval ---
// Make sure they don't work when `relative: "self"`.
// Hint: 17-61 make sure to set `relative: auto` on your text fill
// Error: 17-61 gradients and tilings on text must be relative to the parent
#set text(fill: gradient.linear(red, blue, relative: "self"))

--- gradient-text-global paged ---
// Test that gradient fills on text work for globally defined gradients.
#set page(width: 200pt, height: auto, margin: 10pt, background: {
  rect(width: 100%, height: 30pt, fill: gradient.linear(red, blue))
})
#set par(justify: true)
#set text(fill: gradient.linear(red, blue))
#lorem(30)

--- gradient-text-dir paged ---
// Sanity check that the direction works on text.
#set page(width: 200pt, height: auto, margin: 10pt, background: {
  rect(height: 100%, width: 30pt, fill: gradient.linear(dir: btt, red, blue))
})
#set par(justify: true)
#set text(fill: gradient.linear(dir: btt, red, blue))
#lorem(30)

--- gradient-text-in-container paged ---
// Test that gradient fills on text work for locally defined gradients.
#set page(width: auto, height: auto, margin: 10pt)
#show box: set text(fill: gradient.linear(..color.map.rainbow))
Hello, #box[World]!

--- gradient-text-rotate paged ---
// Test that gradients fills on text work with transforms.
#set page(width: auto, height: auto, margin: 10pt)
#show box: set text(fill: gradient.linear(..color.map.rainbow))
#rotate(45deg, box[World])

--- gradient-text-decoration paged ---
#set text(fill: gradient.linear(red, blue))

Hello #underline[World]! \
Hello #overline[World]! \
Hello #strike[World]! \

--- gradient-transformed paged ---
// Test whether gradients work well when they are contained within a transform.
#let grad = gradient.linear(red, blue, green, purple, relative: "parent");
#let my-rect = rect(width: 50pt, height: 50pt, fill: grad)
#set page(
  height: 50pt,
  width: 50pt,
  margin: 2.5pt,
)
#place(top + right, scale(x: 200%, y: 130%, my-rect))
#place(bottom + center, rotate(45deg, my-rect))
#place(horizon + center, scale(x: 200%, y: 130%, rotate(45deg, my-rect)))

--- gradient-presets paged ---
// Test all gradient presets.
#set page(width: 100pt, height: auto, margin: 0pt)
#set text(fill: white, size: 18pt)
#set text(top-edge: "bounds", bottom-edge: "bounds")

#let presets = (
  ("turbo", color.map.turbo),
  ("cividis", color.map.cividis),
  ("rainbow", color.map.rainbow),
  ("spectral", color.map.spectral),
  ("viridis", color.map.viridis),
  ("inferno", color.map.inferno),
  ("magma", color.map.magma),
  ("plasma", color.map.plasma),
  ("rocket", color.map.rocket),
  ("mako", color.map.mako),
  ("coolwarm", color.map.coolwarm),
  ("vlag", color.map.vlag),
  ("icefire", color.map.icefire),
  ("flare", color.map.flare),
  ("crest", color.map.crest),
)

#stack(
  spacing: 3pt,
  ..presets.map(((name, preset)) => block(
    width: 100%,
    height: 20pt,
    fill: gradient.linear(..preset),
    align(center + horizon, smallcaps(name)),
  ))
)

// Test that gradients are applied correctly on equations.

--- gradient-math-cancel paged ---
// Test on cancel
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow))
#show math.equation: box

$ a dot cancel(5) = cancel(25) 5 x + cancel(5) 1 $

--- gradient-math-frac paged ---
// Test on frac
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow))
#show math.equation: box

$ nabla dot bold(E) = frac(rho, epsilon_0) $

--- gradient-math-root paged ---
// Test on root
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow))
#show math.equation: box

$ x_"1,2" = frac(-b plus.minus sqrt(b^2 - 4 a c), 2 a) $

--- gradient-math-mat paged ---
// Test on matrix
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow))
#show math.equation: box

$ A = mat(
  1, 2, 3;
  4, 5, 6;
  7, 8, 9
) $

--- gradient-math-underover paged ---
// Test on underover
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow))
#show math.equation: box

$ underline(X^2) $
$ overline("hello, world!") $

--- gradient-math-dir paged ---
// Test a different direction
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow, dir: ttb))
#show math.equation: box

$ A = mat(
  1, 2, 3;
  4, 5, 6;
  7, 8, 9
) $

$ x_"1,2" = frac(-b plus.minus sqrt(b^2 - 4 a c), 2 a) $

--- gradient-math-misc paged ---
// Test miscellaneous
#show math.equation: set text(fill: gradient.linear(..color.map.rainbow))
#show math.equation: box

$ hat(x) = bar x bar = vec(x, y, z) = tilde(x) = dot(x) $
$ x prime = vec(1, 2, delim: "[") $
$ sum_(i in NN) 1 + i $
$ attach(
  Pi, t: alpha, b: beta,
  tl: 1, tr: 2+3, bl: 4+5, br: 6,
) $

--- gradient-math-radial paged ---
// Test radial gradient
#show math.equation: set text(fill: gradient.radial(..color.map.rainbow, center: (30%, 30%)))
#show math.equation: box

$ A = mat(
  1, 2, 3;
  4, 5, 6;
  7, 8, 9
) $

--- gradient-math-conic paged ---
// Test conic gradient
#show math.equation: set text(fill: gradient.conic(red, blue, angle: 45deg))
#show math.equation: box

$ A = mat(
  1, 2, 3;
  4, 5, 6;
  7, 8, 9
) $


--- gradient-kind eval ---
// Test gradient functions.
#test(gradient.linear(red, green, blue).kind(), gradient.linear)

--- gradient-stops eval ---
#test(gradient.linear(red, green, blue, space: rgb).stops(), ((red, 0%), (green, 50%), (blue, 100%)))

--- gradient-sample eval ---
#test(gradient.linear(red, green, blue, space: rgb).sample(0%), red)
#test(gradient.linear(red, green, blue, space: rgb).sample(25%), rgb("#97873b"))
#test(gradient.linear(red, green, blue, space: rgb).sample(50%), green)
#test(gradient.linear(red, green, blue, space: rgb).sample(75%), rgb("#17a08c"))
#test(gradient.linear(red, green, blue, space: rgb).sample(100%), blue)

--- gradient-space eval ---
#test(gradient.linear(red, green, space: rgb).space(), rgb)
#test(gradient.linear(red, green, space: oklab).space(), oklab)
#test(gradient.linear(red, green, space: oklch).space(), oklch)
#test(gradient.linear(red, green, space: cmyk).space(), cmyk)
#test(gradient.linear(red, green, space: luma).space(), luma)
#test(gradient.linear(red, green, space: color.linear-rgb).space(), color.linear-rgb)
#test(gradient.linear(red, green, space: color.hsl).space(), color.hsl)
#test(gradient.linear(red, green, space: color.hsv).space(), color.hsv)

--- gradient-relative eval ---
#test(gradient.linear(red, green, relative: "self").relative(), "self")
#test(gradient.linear(red, green, relative: "parent").relative(), "parent")
#test(gradient.linear(red, green).relative(), auto)

--- gradient-angle eval ---
#test(gradient.linear(red, green).angle(), 0deg)
#test(gradient.linear(red, green, dir: ltr).angle(), 0deg)
#test(gradient.linear(red, green, dir: rtl).angle(), 180deg)
#test(gradient.linear(red, green, dir: ttb).angle(), 90deg)
#test(gradient.linear(red, green, dir: btt).angle(), 270deg)

--- gradient-repeat eval ---
#test(
  gradient.linear(red, green, blue, space: rgb).repeat(2).stops(),
  ((red, 0%), (green, 25%), (blue, 50%), (red, 50%), (green, 75%), (blue, 100%))
)
#test(
  gradient.linear(red, green, blue, space: rgb).repeat(2, mirror: true).stops(),
  ((red, 0%), (green, 25%), (blue, 50%), (green, 75%), (red, 100%))
)

--- issue-2902-gradient-oklch-panic paged ---
// Minimal reproduction of #2902
#set page(width: 15cm, height: auto, margin: 1em)
#set block(width: 100%, height: 1cm, above: 2pt)

// Oklch
#block(fill: gradient.linear(red, purple, space: oklch))
#block(fill: gradient.linear(..color.map.rainbow, space: oklch))
#block(fill: gradient.linear(..color.map.plasma, space: oklch))

--- issue-2902-gradient-oklab-panic paged ---
#set page(width: 15cm, height: auto, margin: 1em)
#set block(width: 100%, height: 1cm, above: 2pt)

// Oklab
#block(fill: gradient.linear(red, purple, space: oklab))
#block(fill: gradient.linear(..color.map.rainbow, space: oklab))
#block(fill: gradient.linear(..color.map.plasma, space: oklab))

--- issue-5819-gradient-repeat eval ---
// Ensure the gradient constructor generates monotonic stops which can be fed
// back into the gradient constructor itself.
#let my-gradient = gradient.linear(red, blue).repeat(5)
#let _ = gradient.linear(..my-gradient.stops())
#let my-gradient2 = gradient.linear(red, blue).repeat(5, mirror: true)
#let _ = gradient.linear(..my-gradient2.stops())

--- issue-6162-coincident-gradient-stops-export-png paged ---
// Ensure that multiple gradient stops with the same position
// don't cause a panic.
#rect(
  fill: gradient.linear(
    (red, 0%),
    (green, 0%),
    (blue, 100%),
  )
)
#rect(
  fill: gradient.linear(
    (red, 0%),
    (green, 100%),
    (blue, 100%),
  )
)
#rect(
  fill: gradient.linear(
    (white, 0%),
    (red, 50%),
    (green, 50%),
    (blue, 100%),
  )
)

--- issue-6680-gradient-linear-with-aspect-correction paged ---
#set page(width: 200pt, height: auto, margin: 10pt, fill: gradient.linear(red, blue, angle: 45deg).sharp(2))

--- issue-6597-gradient-angle-negative-size paged ---
#set rect(fill: gradient.linear(angle: 45deg, ..color.map.viridis))
#grid(columns: (1cm, 1cm), rows: 5mm, gutter: 1mm, align: center + horizon,
  rect(width: +1cm, height: +5mm), rect(width: -1cm, height: +5mm),
  rect(width: +1cm, height: -5mm), rect(width: -1cm, height: -5mm),
)

--- gradient-angle-rotation-aspect paged ---
#grid(columns: (1cm, 1cm), rows: 5mm, gutter: 1mm, align: center + horizon,
  rect(width: 1cm, height: 5mm, fill: gradient.linear(angle: 60deg, ..color.map.spectral)),
  rect(width: 1cm, height: 5mm, fill: gradient.linear(angle: 120deg, ..color.map.spectral)),
  rect(width: 1cm, height: 5mm, fill: gradient.linear(angle: 300deg, ..color.map.spectral)),
  rect(width: 1cm, height: 5mm, fill: gradient.linear(angle: 240deg, ..color.map.spectral)),
)

--- gradient-negative-square-angles paged ---
#grid(columns: 12, ..for i in range(-24,24){(
  rect(width: 3mm, height: 3mm, fill: gradient.linear(yellow, black, angle: i * 15deg).sharp(3)),
)})

--- gradient-line-stroke paged ---
#set page(width: auto, height: auto, margin: 5pt)
#let test = g => {
  box(width: 100pt, height: 100pt, {
    place(dx: 40pt, dy: 40pt, circle(radius: 10pt, fill: g))
    for i in range(0,12){
      place(dx: 50pt + 12pt*calc.cos(i*30deg), dy: 50pt + 12pt*calc.sin(i*30deg),
        line(length: 36pt, angle: i*30deg, stroke: g + 10pt)
      )
    }
  })
}
#grid(columns: 4,
  ..(0deg, 30deg, 45deg, 90deg, 180deg, -125deg).map(
    a => test(gradient.linear(yellow, black, angle: a).sharp(4))),
  test(gradient.radial(blue, orange).sharp(4)),
  test(gradient.conic(..color.map.spectral).sharp(12)),
)

--- gradient-curve-stroke paged ---
#set page(width: auto, height: auto, margin: 5pt)
#let test = g => {
  box(width: 100pt, height: 100pt, {
    place(dx: 40pt, dy: 40pt, circle(radius: 10pt, fill: g))
    for i in range(0,12){
      place(dx: 50pt + 12pt*calc.cos(i*30deg), dy: 50pt + 12pt*calc.sin(i*30deg),
        curve(
          curve.line((36pt * calc.cos(i*30deg), 36pt * calc.sin(i*30deg))),
          stroke: g + 10pt,
        )
      )
    }
  })
}
#grid(columns: 4,
  ..(0deg, 30deg, 45deg, 90deg, 180deg, -125deg).map(
    a => test(gradient.linear(yellow, black, angle: a).sharp(4))),
  test(gradient.radial(blue, orange).sharp(4)),
  test(gradient.conic(..color.map.spectral).sharp(12)),
)

--- gradient-curve-stroke-quad paged ---
#set page(width: auto, height: auto, margin: 5pt)
#let test = g => {
  box(width: 100pt, height: 100pt, {
    place(dx: 40pt, dy: 40pt, circle(radius: 10pt, fill: g))
    for i in range(0,12){
      place(dx: 50pt + 12pt*calc.cos(i*30deg), dy: 50pt + 12pt*calc.sin(i*30deg),
        curve(
          curve.quad((18pt * calc.cos((i+1)*30deg), 18pt * calc.sin((i+1)*30deg)), (36pt * calc.cos(i*30deg), 36pt * calc.sin(i*30deg))),
          stroke: g + 10pt,
        )
      )
    }
  })
}
#grid(columns: 4,
  ..(0deg, 30deg, 45deg, 90deg, 180deg, -125deg).map(
    a => test(gradient.linear(yellow, black, angle: a).sharp(4))),
  test(gradient.radial(blue, orange).sharp(4)),
  test(gradient.conic(..color.map.spectral).sharp(12)),
)

--- gradient-line-cap paged ---
// Test that line caps are taken into account for gradient fills.
#for cap in ("square", "butt", "round"){
  box(line(length: 10pt, stroke: (
    thickness: 20pt,
    paint: gradient.radial(blue, orange).sharp(4),
    cap: cap
  )), width: 30pt)
}

--- issue-6068-curve-stroke-gradient paged ---
#let stroke = gradient.linear(blue, red).sharp(2)
#line(start: (0pt, 0pt), end: (100pt, 0pt), stroke: stroke)
#curve(curve.line((100pt, 0pt)), stroke: stroke)
#curve(curve.quad(none, (100pt, 0pt)), stroke: stroke)
#curve(
  curve.quad(none, (100pt, 0pt)),
  curve.quad(none, (100pt, 10pt)),
  stroke: stroke
)
#line(start: (10pt, 0pt), end: (90pt, 0pt), stroke: stroke)
#curve(
  curve.move((10pt, 0pt)),
  curve.quad(none, (90pt, 0pt)),
  curve.quad(none, (90pt, 10pt)),
  stroke: stroke
)

--- issue-5705-gradient-border-stroke paged ---
#let stroke = (
  top: gradient.linear(green, black, yellow, angle: 90deg) + 15pt,
  right: gradient.linear(green, black, yellow, angle: 180deg) + 15pt,
  bottom: gradient.linear(green, black, yellow, angle: 270deg) + 15pt,
  left: gradient.linear(green, black, yellow, angle: 0deg) + 15pt,
)
#set align(center + horizon)
#rect(width: 100pt, height: 100pt, radius: 15pt, stroke: stroke,
  rect(width: 65pt, height: 65pt, radius: 0pt, stroke: stroke,
    rect(width: 30pt, height: 30pt, radius: 30pt, stroke: stroke)
  )
)

--- gradient-spot-color-same-colorant paged ---
// Test gradient with spot colors of the same colorant (should interpolate tint)
#let pantone = color.spot("PANTONE 2221 C", eastern)
#set page(width: 100pt, height: 30pt, margin: 0pt)
#let g = gradient.linear(
  pantone.tint(20%),
  pantone.tint(100%),
  space: pantone,
)
#test(g.sample(10%).space(), pantone)
#block(
  width: 100%,
  height: 100%,
  fill: g,
)

--- gradient-spot-color-different-colorant eval ---
// Test gradient with spot colors of a different colorant (should error)
#let pantone = color.spot("PANTONE 185 C", rgb(89.4%, 0.7%, 17%))
#let ral = color.spot("RAL 1023", rgb("#F0CA00"))
#let g = gradient.linear(
  pantone.tint(20%),
  // Error: 3-17 cannot convert spot color to different spot colorant
  // Hint: 3-17 colorant of the color is `"RAL 1023"`
  // Hint: 3-17 colorant of the color space is `"PANTONE 185 C"`
  ral.tint(100%),
  // Hint: 10-17 gradient color mixing space specified here
  space: pantone,
)

--- gradient-spot-color-to-process paged ---
// Test gradient with spot color mixed with process color (uses fallback in process space)
#let pantone = color.spot("PANTONE 185 C", rgb(89.4%, 0.7%, 17%))
#set page(width: 100pt, height: 30pt, margin: 0pt)
#block(
  width: 100%,
  height: 100%,
  fill: gradient.linear(
    pantone.tint(80%),
    blue,
    space: rgb,
  ),
)
//@@ file tests/suite/visualize_image.typ
// Test the `image` function.

--- image-png paged ---
// Load an RGBA PNG image.
#image("/assets/images/rhino.png")

--- image-jpg paged ---
// Load an RGB JPEG image.
#set page(height: 60pt)
#image("/assets/images/tiger.jpg")

--- image-jpg-html-base64 html ---
#image("/assets/images/f2t.jpg", alt: "The letter F")

--- image-sizing-html-css html ---
#image("/assets/images/f2t.jpg", width: 50%, alt: "width: 50%")
#image("/assets/images/f2t.jpg", width: 100pt, alt: "width: 100pt")
#image("/assets/images/f2t.jpg", width: 30% + 50pt, alt: "width: calc(30% + 50pt)")
#image("/assets/images/f2t.jpg", height: 75%, alt: "height: 75%")
#image("/assets/images/f2t.jpg", height: 80pt, alt: "height: 80pt")
#image("/assets/images/f2t.jpg", height: 20% + 40pt, alt: "height: calc(20% + 40pt)")

--- image-blocky-html html ---
// Ensure that the image does not end up inline (the HTML `img` element is
// phrasing content and `display: inline` by default).
A paragraph followed by an image:
#image("/assets/images/f2t.jpg")

--- image-sizing paged ---
// Test configuring the size and fitting behaviour of images.

// Set width and height explicitly.
#box(image("/assets/images/rhino.png", width: 30pt))
#box(image("/assets/images/rhino.png", height: 30pt))

// Set width and height explicitly and force stretching.
#image("/assets/images/monkey.svg", width: 100%, height: 20pt, fit: "stretch")

// Make sure the bounding-box of the image is correct.
#align(bottom + right, image("/assets/images/tiger.jpg", width: 40pt, alt: "A tiger"))

--- image-fit paged ---
// Test all three fit modes.
#set page(height: 50pt, margin: 0pt)
#grid(
  columns: (1fr, 1fr, 1fr),
  rows: 100%,
  gutter: 3pt,
  image("/assets/images/tiger.jpg", width: 100%, height: 100%, fit: "contain"),
  image("/assets/images/tiger.jpg", width: 100%, height: 100%, fit: "cover"),
  image("/assets/images/monkey.svg", width: 100%, height: 100%, fit: "stretch"),
)

--- image-jump-to-next-page paged ---
// Does not fit to remaining height of page.
#set page(height: 60pt)
Stuff
#image("/assets/images/rhino.png")

--- image-baseline-with-box paged ---
// Test baseline.
A #box(image("/assets/images/tiger.jpg", height: 1cm, width: 80%)) B

--- image-svg-complex paged ---
// Test advanced SVG features.
#image("/assets/images/pattern.svg")

--- image-svg-text paged ---
#set page(width: 250pt)

#figure(
  image("/assets/images/diagram.svg"),
  caption: [A textful diagram],
)

--- image-svg-text-font paged ---
#set page(width: 250pt)
#show image: set text(font: ("Roboto", "Noto Serif CJK SC"))

#figure(
  image("/assets/images/chinese.svg"),
  caption: [Bilingual text]
)

--- image-svg-variable-font paged ---
#image(bytes(
  ```
  <svg id="svg1" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
    <text
      x="10" y="40" font-family="Cantarell" font-size="32"
      style="font-variation-settings: 'wght' 300"
    >
      Hello
    </text>
    <text
      x="10" y="80" font-family="Cantarell" font-size="32"
      style="font-variation-settings: 'wght' 700"
    >
      Hello
    </text>
  </svg>
  ```.text
))

--- image-svg-auto-detection paged ---
#image(bytes(
  ```
  <?xml version="1.0" encoding="utf-8"?>
  <!-- An SVG -->
  <svg width="200" height="150" xmlns="http://www.w3.org/2000/svg">
    <rect fill="red" stroke="black" x="25" y="25" width="150" height="100"/>
  </svg>
  ```.text
))

--- image-svg-foreign-object-warning paged ---
// Warning: 1:2-9:3 image contains foreign object
// Hint: 1:2-9:3 its content will be omitted because Typst cannot render embedded HTML
// Hint: 1:2-9:3 see https://github.com/typst/typst/issues/1421 for more information
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg" width="200" height="150">
    <foreignObject width="200" height="150">
      <div xmlns="http://www.w3.org/1999/xhtml">Rich HTML content</div>
    </foreignObject>
  </svg>
  ```.text
))

--- image-svg-foreign-object-inside-switch-svg2-0 paged ---
// SVG 2.0 style, using requiredExtensions
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg" width="200" height="150">
    <switch>
      <foreignObject requiredExtensions="http://www.w3.org/1999/xhtml" width="200" height="150">
        <div xmlns="http://www.w3.org/1999/xhtml">Rich HTML content</div>
      </foreignObject>
      <text x="10" y="80" font-size="16">fallback text</text>
    </switch>
  </svg>
  ```.text
))

--- image-svg-foreign-object-inside-switch-svg1-1 paged ---
// SVG 1.1 style, using requiredFeatures, remaining in draw.io output
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg" width="200" height="150">
    <switch>
      <foreignObject requiredFeatures="http://www.w3.org/TR/SVG11/feature#Extensibility" width="200" height="150">
        <div xmlns="http://www.w3.org/1999/xhtml">Rich HTML content</div>
      </foreignObject>
      <text x="10" y="80" font-size="16">fallback text</text>
    </switch>
  </svg>
  ```.text
))

--- image-svg-linked-jpg1 paged ---
#set page(fill: gray)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg" height="80" width="48">
    <image href="../../../assets/images/f2t.jpg" />
    <circle r="32" cx="24" cy="40" fill="none" stroke="red" />
  </svg>
  ```.text
))

--- image-svg-linked-jpg2 paged ---
#set page(fill: gray)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg" height="80" width="48">
    <image href="file://../../../assets/images/f2t.jpg" />
    <circle r="32" cx="24" cy="40" fill="none" stroke="blue" />
  </svg>
  ```.text
))

--- image-svg-linked-many-formats paged ---
#set page(width: auto, height: auto, margin: 1pt)
#set text(1pt)
#image("/assets/images/linked.svg", width: 39pt)

--- image-svg-linked-file-not-found paged ---
// Error: 1:8-7:2 failed to load linked image do-not-add-image-with-this-name.png in SVG (file not found, searched at tests/suite/visualize/do-not-add-image-with-this-name.png)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="do-not-add-image-with-this-name.png" />
  </svg>
  ```.text
))

--- image-svg-linked-url paged ---
// Error: 1:8-7:2 failed to load linked image https://somedomain.com/image.png in SVG (URLs are not allowed)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="https://somedomain.com/image.png" />
  </svg>
  ```.text
))

--- image-svg-linked-pdf paged ---
// Error: 1:8-7:2 failed to load linked image ../../../assets/images/diagrams.pdf in SVG (PDF documents are not supported)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="../../../assets/images/diagrams.pdf" />
  </svg>
  ```.text
))

--- image-svg-linked-csv paged ---
// Error: 1:8-7:2 failed to load linked image ../../../assets/data/bad.csv in SVG (unknown image format)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="../../../assets/data/bad.csv" />
  </svg>
  ```.text
))

--- image-svg-linked-absolute1 paged ---
// Error: 1:8-7:2 failed to load linked image /home/user/foo.svg in SVG (absolute paths are not allowed)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="/home/user/foo.svg" />
  </svg>
  ```.text
))

--- image-svg-linked-absolute2 paged ---
// Error: 1:8-7:2 failed to load linked image file:///home/user/foo.svg in SVG (absolute paths are not allowed)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="file:///home/user/foo.svg" />
  </svg>
  ```.text
))

--- image-svg-linked-invalid paged ---
// Error: 1:8-7:2 failed to load linked image ../../../assets/images/bad.svg in SVG (failed to parse SVG)
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg">
    <image href="../../../assets/images/bad.svg" />
  </svg>
  ```.text
))

--- image-pixmap-rgb8 paged ---
#image(
  bytes((
    0xFF, 0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0xFF,
    0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x80,
    0x80, 0x80, 0x00, 0x00, 0x80, 0x80, 0x80, 0x00, 0x80,
  )),
  format: (
    encoding: "rgb8",
    width: 3,
    height: 3,
  ),
  width: 1cm,
)

--- image-pixmap-rgba8 paged ---
#image(
  bytes((
    0xFF, 0x00, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0x00, 0xFF, 0xFF,
    0xFF, 0x00, 0x00, 0x80, 0x00, 0xFF, 0x00, 0x80, 0x00, 0x00, 0xFF, 0x80,
    0xFF, 0x00, 0x00, 0x10, 0x00, 0xFF, 0x00, 0x10, 0x00, 0x00, 0xFF, 0x10,
  )),
  format: (
    encoding: "rgba8",
    width: 3,
    height: 3,
  ),
  width: 1cm,
)

--- image-pixmap-luma8 paged ---
#image(
  bytes(range(16).map(x => x * 16)),
  format: (
    encoding: "luma8",
    width: 4,
    height: 4,
  ),
  width: 1cm,
)

--- image-pixmap-lumaa8 paged ---
#image(
  bytes(range(16).map(x => (0x80, x * 16)).flatten()),
  format: (
    encoding: "lumaa8",
    width: 4,
    height: 4,
  ),
  width: 1cm,
)

--- image-scaling-methods paged html ---
#let img(scaling) = image(
  bytes((
    0xFF, 0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0xFF,
    0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x80,
    0x80, 0x80, 0x00, 0x00, 0x80, 0x80, 0x80, 0x00, 0x80,
  )),
  format: (
    encoding: "rgb8",
    width: 3,
    height: 3,
  ),
  width: 1cm,
  height: 1cm,
  scaling: scaling,
)

#let images = (
  img(auto),
  img("smooth"),
  img("pixelated"),
)

#context if target() == "html" {
  // TODO: Remove this once `stack` is supported in HTML export.
  html.div(
    style: "display: flex; flex-direction: row; gap: 4pt",
    images.join(),
  )
} else {
  stack(
    dir: ltr,
    spacing: 4pt,
    ..images,
  )
}

--- image-natural-dpi-sizing paged ---
// Test that images aren't upscaled.
// Image is just 48x80 at 220dpi. It should not be scaled to fit the page
// width, but rather max out at its natural size.
#image("/assets/images/f2t.jpg")

--- image-file-not-found eval ---
// Error: 8-29 file not found (searched at tests/suite/visualize/path/does/not/exist)
#image("path/does/not/exist")

--- image-bad-format paged ---
// Error: 2-37 unknown image format
#image("/assets/plugins/hello.wasm")

--- image-bad-svg paged ---
// Error: "/assets/images/bad.svg" 4:3 failed to parse SVG (found closing tag 'g' instead of 'style')
#image("/assets/images/bad.svg")

--- image-decode-svg paged ---
// Test parsing from svg data
#image(bytes(`<svg xmlns="http://www.w3.org/2000/svg" height="140" width="500"><ellipse cx="200" cy="80" rx="100" ry="50" style="fill:yellow;stroke:purple;stroke-width:2" /></svg>`.text), format: "svg")

--- image-decode-bad-svg paged ---
// Error: 8-150 failed to parse SVG (missing root node at 1:1)
#image(bytes(`<sv height="140" width="500"><ellipse cx="200" cy="80" rx="100" ry="50" style="fill:yellow;stroke:purple;stroke-width:2" /></sv>`.text), format: "svg")

--- image-decode-detect-format paged ---
// Test format auto detect
#image(read("/assets/images/tiger.jpg", encoding: none), width: 80%)

--- image-decode-specify-format paged ---
// Test format manual
#image(read("/assets/images/tiger.jpg", encoding: none), format: "jpg", width: 80%)

--- image-decode-specify-wrong-format paged ---
// Error: 2-84 failed to decode image (Format error decoding Png: Invalid PNG signature.)
#image(read("/assets/images/tiger.jpg", encoding: none), format: "png", width: 80%)

--- image-pixmap-empty paged ---
// Error: 1:2-8:2 zero-sized images are not allowed
#image(
  bytes(()),
  format: (
    encoding: "rgb8",
    width: 0,
    height: 0,
  ),
)

--- image-pixmap-invalid-size paged ---
// Error: 1:2-8:2 pixel dimensions and pixel data do not match
#image(
  bytes((0x00, 0x00, 0x00)),
  format: (
    encoding: "rgb8",
    width: 16,
    height: 16,
  ),
)

--- image-pixmap-unknown-attribute eval ---
#image(
  bytes((0x00, 0x00, 0x00)),
  // Error: 1:11-6:4 unexpected key "stowaway", valid keys are "encoding", "width", and "height"
  format: (
    encoding: "rgb8",
    width: 1,
    height: 1,
    stowaway: "I do work here, promise",
  ),
)

--- image-pixmap-but-png-format eval ---
#image(
  bytes((0x00, 0x00, 0x00)),
  // Error: 1:11-5:4 expected "rgb8", "rgba8", "luma8", or "lumaa8"
  format: (
    encoding: "png",
    width: 1,
    height: 1,
  ),
)

--- image-png-but-pixmap-format eval ---
#image(
  read("/assets/images/tiger.jpg", encoding: none),
  // Error: 11-18 expected "png", "jpg", "gif", "webp", dictionary, "svg", "pdf", or auto
  format: "rgba8",
)

--- issue-measure-image paged empty ---
// Test that image measurement doesn't turn `inf / some-value` into 0pt.
#context {
  let size = measure(image("/assets/images/tiger.jpg"))
  test(size, (width: 1024pt, height: 670pt, baseline: 670pt))
}

--- issue-2051-new-cm-svg paged ---
#set text(font: "New Computer Modern")
#image("/assets/images/diagram.svg")

--- issue-3733-dpi-svg paged ---
#set page(width: 200pt, height: 200pt, margin: 0pt)
#image("/assets/images/relative.svg")

--- image-exif-rotation paged ---
#let rotations = range(1, 9)
#let with-rotation(path, offset, v) = {
  let data = read(path, encoding: none)
  let modified = data.slice(0, offset) + bytes((v,)) + data.slice(offset + 1)
  image(modified, width: 10pt)
}

#set page(width: auto)
#table(
  columns: 1 + rotations.len(),
  table.header(
    [], ..rotations.map(v => raw(str(v), lang: "typc")),
  ),
  `PNG`, ..rotations.map(v => with-rotation("/assets/images/f2t.png", 0x85, v)),
  // JPEG has special handing in PDF export (no recoding, so instead we use a
  // transform to apply the orientation), so it's worth testing that separately.
  `JPEG`, ..rotations.map(v => with-rotation("/assets/images/f2t.jpg", 0x31, v)),
)

--- image-pdf-basic paged html ---
#image("/assets/images/star.pdf")

--- image-pdf-complex paged tolerance(2) ---
#image("/assets/images/matplotlib.pdf")

--- image-pdf-multiple-pages paged ---
#image("/assets/images/diagrams.pdf", page: 1)
#image("/assets/images/diagrams.pdf", page: 3)
#image("/assets/images/diagrams.pdf", page: 2)

--- image-pdf-base14-fonts paged ---
// Test PDF base 14 fonts.
#image("/assets/images/base14-fonts.pdf")

--- image-pdf-invalid-page paged ---
// Error: 8-39 failed to load PDF (page 2 does not exist in assets/images/matplotlib.pdf)
// Hint: 8-39 the document only has 1 page
#image("/assets/images/matplotlib.pdf", page: 2)

--- issue-6869-image-zero-sized paged ---
// Primarily to ensure that it does not crash in PDF export.
#image("/assets/images/f2t.jpg", width: 0pt, height: 0pt)

--- issue-7178-svg-fallback-deadlock paged ---
// We used to not honor resvg's `exclude_fonts` mechanism, which could result in
// an infinite fallback loop.
//
// On way to trigger this is if there are two codepoints that result in one
// cluster, and the first exists in a font, but the second always shapes to a
// tofu.
#image(bytes(
  ```
  <svg xmlns="http://www.w3.org/2000/svg" height="1" width="1">
    <text font-family="Libertinus Serif">x&#1761;</text>
  </svg>
  ```.text
))
//@@ file tests/suite/visualize_line.typ
// Test lines.

--- line-basic paged ---
#set page(height: 60pt)
#box({
  set line(stroke: 0.75pt)
  place(line(end: (0.4em, 0pt)))
  place(line(start: (0pt, 0.4em), end: (0pt, 0pt)))
  line(end: (0.6em, 0.6em))
}) Hello #box(line(length: 1cm))!

#line(end: (70%, 50%))

--- line-positioning paged ---
// Test the angle argument and positioning.

#set page(fill: rgb("0B1026"))
#set line(stroke: white)

#let star(size, ..args) = box(width: size, height: size)[
  #set text(spacing: 0%)
  #set line(..args)
  #set align(left)
  #v(30%)
  #place(line(length: +30%, start: (09.0%, 02%)))
  #place(line(length: +30%, start: (38.7%, 02%), angle: -72deg))
  #place(line(length: +30%, start: (57.5%, 02%), angle: 252deg))
  #place(line(length: +30%, start: (57.3%, 02%)))
  #place(line(length: -30%, start: (88.0%, 02%), angle: -36deg))
  #place(line(length: +30%, start: (73.3%, 48%), angle: 252deg))
  #place(line(length: -30%, start: (73.5%, 48%), angle: 36deg))
  #place(line(length: +30%, start: (25.4%, 48%), angle: -36deg))
  #place(line(length: +30%, start: (25.6%, 48%), angle: -72deg))
  #place(line(length: +32%, start: (8.50%, 02%), angle: 34deg))
]

#align(center, grid(
  columns: 3,
  column-gutter: 10pt,
  ..((star(20pt, stroke: 0.5pt),) * 9)
))

--- line-stroke paged ---
// Some simple test lines
#line(length: 60pt, stroke: red)
#v(3pt)
#line(length: 60pt, stroke: 2pt)
#v(3pt)
#line(length: 60pt, stroke: blue + 1.5pt)
#v(3pt)
#line(length: 60pt, stroke: (paint: red, thickness: 1pt, dash: "dashed"))
#v(3pt)
#line(length: 60pt, stroke: (paint: red, thickness: 4pt, cap: "round"))

--- line-stroke-set paged ---
// Set rules with stroke
#set line(stroke: (paint: red, thickness: 1pt, cap: "butt", dash: "dash-dotted"))
#line(length: 60pt)
#v(3pt)
#line(length: 60pt, stroke: blue)
#v(3pt)
#line(length: 60pt, stroke: (dash: none))

--- line-stroke-dash paged ---
// Dashing
#line(length: 60pt, stroke: (paint: red, thickness: 1pt, dash: ("dot", 1pt)))
#v(3pt)
#line(length: 60pt, stroke: (paint: red, thickness: 1pt, dash: ("dot", 1pt, 4pt, 2pt)))
#v(3pt)
#line(length: 60pt, stroke: (paint: red, thickness: 1pt, dash: (array: ("dot", 1pt, 4pt, 2pt), phase: 5pt)))
#v(3pt)
#line(length: 60pt, stroke: (paint: red, thickness: 1pt, dash: ()))
#v(3pt)
#line(length: 60pt, stroke: (paint: red, thickness: 1pt, dash: (1pt, 3pt, 9pt)))

--- line-stroke-field-typo eval ---
// Error: 29-56 unexpected key "thicknes", valid keys are "paint", "thickness", "cap", "join", "dash", and "miter-limit"
#line(length: 60pt, stroke: (paint: red, thicknes: 1pt))

--- line-stroke-bad-dash-kind eval ---
// Error: 29-55 expected "solid", "dotted", "densely-dotted", "loosely-dotted", "dashed", "densely-dashed", "loosely-dashed", "dash-dotted", "densely-dash-dotted", "loosely-dash-dotted", array, dictionary, none, or auto
#line(length: 60pt, stroke: (paint: red, dash: "dash"))

--- line-bad-point-array eval ---
// Test errors.

// Error: 12-19 array must contain exactly two items
// Hint: 12-19 the first item determines the value for the X axis and the second item the value for the Y axis
#line(end: (50pt,))

--- line-bad-point-component-type eval ---
// Error: 14-26 expected relative length, found angle
#line(start: (3deg, 10pt), length: 5cm)

--- line-infinite-length paged ---
// Error: 2-54 cannot create line with infinite length
#line(start: (0pt, 0pt), end: (float.inf * 1pt, 0pt))
//@@ file tests/suite/visualize_polygon.typ
// Test polygons.

--- polygon paged ---
#set page(width: 50pt)
#set polygon(stroke: 0.75pt, fill: blue)

// These are not visible, but should also not give an error
#polygon()
#polygon((0em, 0pt))
#polygon((0pt, 0pt), (10pt, 0pt))
#polygon.regular(size: 0pt, vertices: 9)

#polygon((5pt, 0pt), (0pt, 10pt), (10pt, 10pt))
#polygon(
  (0pt, 0pt), (5pt, 5pt), (10pt, 0pt),
  (15pt, 5pt),
  (5pt, 10pt)
)
#polygon(stroke: none, (5pt, 0pt), (0pt, 10pt), (10pt, 10pt))
#polygon(stroke: 3pt, fill: none, (5pt, 0pt), (0pt, 10pt), (10pt, 10pt))

// Relative size
#polygon((0pt, 0pt), (100%, 5pt), (50%, 10pt))

// Antiparallelogram
#polygon((0pt, 5pt), (5pt, 0pt), (0pt, 10pt), (5pt, 15pt))

// Self-intersections
#polygon((0pt, 10pt), (30pt, 20pt), (0pt, 30pt), (20pt, 0pt), (20pt, 35pt))
#polygon(fill-rule: "non-zero", (0pt, 10pt), (30pt, 20pt), (0pt, 30pt), (20pt, 0pt), (20pt, 35pt))
#polygon(fill-rule: "even-odd", (0pt, 10pt), (30pt, 20pt), (0pt, 30pt), (20pt, 0pt), (20pt, 35pt))

// Regular polygon; should have equal side lengths
#for k in range(3, 9) {polygon.regular(size: 30pt, vertices: k,)}

--- polygon-line-join paged ---
// Line joins
#stack(
  dir: ltr,
  spacing: 1em,
  polygon(stroke: (thickness: 4pt, paint: blue, join: "round"),
    (0pt, 20pt), (15pt, 0pt), (0pt, 40pt), (15pt, 45pt)),
  polygon(stroke: (thickness: 4pt, paint: blue, join: "bevel"),
    (0pt, 20pt), (15pt, 0pt), (0pt, 40pt), (15pt, 45pt)),
  polygon(stroke: (thickness: 4pt, paint: blue, join: "miter"),
    (0pt, 20pt), (15pt, 0pt), (0pt, 40pt), (15pt, 45pt)),
  polygon(stroke: (thickness: 4pt, paint: blue, join: "miter", miter-limit: 20.0),
    (0pt, 20pt), (15pt, 0pt), (0pt, 40pt), (15pt, 45pt)),
)

--- polygon-bad-point-array eval ---
// Error: 10-17 array must contain exactly two items
// Hint: 10-17 the first item determines the value for the X axis and the second item the value for the Y axis
#polygon((50pt,))

--- polygon-infinite-size paged ---
// Error: 2-57 cannot create polygon with infinite size
#polygon((0pt, 0pt), (0pt, 1pt), (float.inf * 1pt, 0pt))
//@@ file tests/suite/visualize_rect.typ
// Test the `rect` function.

--- rect paged ---
// Default rectangle.
#rect()

--- rect-customization paged ---
#set page(width: 150pt)

// Fit to text.
#rect(fill: conifer)[Textbox]

// Empty with fixed width and height.
#block(rect(
  height: 15pt,
  fill: rgb("46b3c2"),
  stroke: 2pt + rgb("234994"),
))

// Fixed width, text height.
#rect(width: 2cm, fill: rgb("9650d6"))[Fixed and padded]

// Page width, fixed height.
#rect(height: 1cm, width: 100%, fill: rgb("734ced"))[Topleft]

// These are inline with text.
{#box(rect(width: 0.5in, height: 7pt, fill: rgb("d6cd67")))
 #box(rect(width: 0.5in, height: 7pt, fill: rgb("edd466")))
 #box(rect(width: 0.5in, height: 7pt, fill: rgb("e3be62")))}

// Rounded corners.
#stack(
  dir: ltr,
  spacing: 1fr,
  rect(width: 2cm, radius: 30%),
  rect(width: 1cm, radius: (left: 10pt, right: 5pt)),
  rect(width: 1.25cm, radius: (
    top-left: 2pt,
    top-right: 5pt,
    bottom-right: 8pt,
    bottom-left: 11pt
  )),
)

// Different strokes.
#set rect(stroke: (right: red))
#rect(width: 100%, fill: lime, stroke: (x: 5pt, y: 1pt))

--- rect-stroke paged ---
// Rectangle strokes
#rect(width: 20pt, height: 20pt, stroke: red)
#v(3pt)
#rect(width: 20pt, height: 20pt, stroke: (rest: red, top: (paint: blue, dash: "dashed")))
#v(3pt)
#rect(width: 20pt, height: 20pt, stroke: (thickness: 5pt, join: "round"))

--- rect-stroke-caps paged ---
// Separated segments
#rect(width: 20pt, height: 20pt, stroke: (
  left: (cap: "round", thickness: 5pt),
  right: (cap: "square", thickness: 7pt),
))
// Joined segment with different caps.
#rect(width: 20pt, height: 20pt, stroke: (
  left: (cap: "round", thickness: 5pt),
  top: (cap: "square", thickness: 7pt),
))
// No caps when there is a radius for that corner.
#rect(width: 20pt, height: 20pt, radius: (top: 3pt), stroke: (
  left: (cap: "round", thickness: 5pt),
  top: (cap: "square", thickness: 7pt),
))
--- red-stroke-bad-type eval ---
// Error: 15-21 expected length, color, gradient, tiling, dictionary, stroke, none, or auto, found array
#rect(stroke: (1, 2))

--- rect-fill-stroke paged ---
#let variant = rect.with(width: 20pt, height: 10pt)
#let items = for (i, item) in (
  variant(stroke: none),
  variant(),
  variant(fill: none),
  variant(stroke: 2pt),
  variant(stroke: eastern),
  variant(stroke: eastern + 2pt),
  variant(fill: eastern),
  variant(fill: eastern, stroke: none),
  variant(fill: forest, stroke: none),
  variant(fill: forest, stroke: conifer),
  variant(fill: forest, stroke: black + 2pt),
  variant(fill: forest, stroke: conifer + 2pt),
).enumerate() {
  (align(horizon)[#(i + 1).], item, [])
}

#grid(
  columns: (auto, auto, 1fr, auto, auto, 0fr),
  gutter: 5pt,
  ..items,
)

--- rect-radius-bad-key eval ---
// Error: 15-38 unexpected key "cake", valid keys are "top-left", "top-right", "bottom-right", "bottom-left", "left", "top", "right", "bottom", and "rest"
#rect(radius: (left: 10pt, cake: 5pt))

--- issue-1825-rect-overflow paged ---
#set page(width: 17.8cm)
#set par(justify: true)
#rect(lorem(70))

--- issue-3264-rect-negative-dimensions paged ---
// Negative dimensions
#rect(width: -1cm, fill: gradient.linear(red, blue))[Reverse left]

#rect(width: 1cm, fill: gradient.linear(red, blue))[Left]

#align(center, rect(width: -1cm, fill: gradient.linear(red, blue))[Reverse center])

#align(center, rect(width: 1cm, fill: gradient.linear(red, blue))[Center])

#align(right, rect(width: -1cm, fill: gradient.linear(red, blue))[Reverse right])

#align(right, rect(width: 1cm, fill: gradient.linear(red, blue))[Right])

--- issue-7901-rect-negative-dimensions-radius paged ---
#let rects = grid(columns: (7mm, 7mm, 7mm, 7mm), gutter: 5pt,
  rect(), rect(width: -7mm), rect(height: -1cm), rect(width: -7mm, height: -1cm)
)
#set rect(fill: red, width: 7mm, height: 1cm, radius: 40%)
#set align(center + horizon)
#rects
#set rect(stroke: black + 3pt)
#rects
#set rect(stroke: (left: black + 3pt, top: blue + 3pt, right: green + 3pt, bottom: yellow + 3pt))
#rects
#set rect(radius: 0pt)
#rects

--- rect-size-beyond-default paged ---
// Test that setting a rectangle's height beyond its default sizes it correctly.
#rect()
#rect(height: 60pt)
#rect(width: 60pt)

--- rect-stroke-variations-without-radius paged ---
#import "block.typ": test-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [butt], [square], [round],
  [no dash],
  test-block(cap: "butt"),
  test-block(cap: "square"),
  test-block(cap: "round"),

  [dashed],
  test-block(cap: "butt", dash: "dashed"),
  test-block(cap: "square", dash: "dashed"),
  test-block(cap: "round", dash: "dashed"),

  [loosely-dashed],
  test-block(cap: "butt", dash: "loosely-dashed"),
  test-block(cap: "square", dash: "loosely-dashed"),
  test-block(cap: "round", dash: "loosely-dashed"),
)

--- rect-stroke-variations-with-radius paged ---
#import "block.typ": test-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [butt], [square], [round],
  [no dash],
  test-block(cap: "butt", radius: 12pt),
  test-block(cap: "square", radius: 12pt),
  test-block(cap: "round", radius: 12pt),

  [dashed],
  test-block(cap: "butt", radius: 12pt, dash: "dashed"),
  test-block(cap: "square", radius: 12pt, dash: "dashed"),
  test-block(cap: "round", radius: 12pt, dash: "dashed"),

  [loosely-dashed],
  test-block(cap: "butt", radius: 12pt, dash: "loosely-dashed"),
  test-block(cap: "square", radius: 12pt, dash: "loosely-dashed"),
  test-block(cap: "round", radius: 12pt, dash: "loosely-dashed"),
)

--- rect-stroke-variations-with-radius-and-adjacent-zero-width-stroke paged ---
#import "block.typ": test-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [butt], [square], [round],
  [no dash],
  test-block(cap: "butt", radius: 12pt, adjacent: 0pt),
  test-block(cap: "square", radius: 12pt, adjacent: 0pt),
  test-block(cap: "round", radius: 12pt, adjacent: 0pt),

  [dashed],
  test-block(cap: "butt", radius: 12pt, adjacent: 0pt, dash: "dashed"),
  test-block(cap: "square", radius: 12pt, adjacent: 0pt, dash: "dashed"),
  test-block(cap: "round", radius: 12pt, adjacent: 0pt, dash: "dashed"),

  [loosely-dashed],
  test-block(cap: "butt", radius: 12pt, adjacent: 0pt, dash: "loosely-dashed"),
  test-block(cap: "square", radius: 12pt, adjacent: 0pt, dash: "loosely-dashed"),
  test-block(cap: "round", radius: 12pt, adjacent: 0pt, dash: "loosely-dashed"),
)

--- rect-stroke-variations-with-radius-and-adjacent-thin-stroke paged ---
#import "block.typ": test-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [butt], [square], [round],
  [no dash],
  test-block(cap: "butt", radius: 12pt, adjacent: 1pt),
  test-block(cap: "square", radius: 12pt, adjacent: 1pt),
  test-block(cap: "round", radius: 12pt, adjacent: 1pt),

  [dashed],
  test-block(cap: "butt", radius: 12pt, adjacent: 1pt, dash: "dashed"),
  test-block(cap: "square", radius: 12pt, adjacent: 1pt, dash: "dashed"),
  test-block(cap: "round", radius: 12pt, adjacent: 1pt, dash: "dashed"),

  [loosely-dashed],
  test-block(cap: "butt", radius: 12pt, adjacent: 1pt, dash: "loosely-dashed"),
  test-block(cap: "square", radius: 12pt, adjacent: 1pt, dash: "loosely-dashed"),
  test-block(cap: "round", radius: 12pt, adjacent: 1pt, dash: "loosely-dashed"),
)

--- rect-cap-variations-radius-4-3-of-stroke-thickness paged ---
#import "block.typ": another-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [none], [0 width], [thin],
  [butt],
  another-block(cap: "butt", radius: 8pt, adjacent: none),
  another-block(cap: "butt", radius: 8pt, adjacent: 0pt),
  another-block(cap: "butt", radius: 8pt, adjacent: 1pt),

  [square],
  another-block(cap: "square", radius: 8pt, adjacent: none),
  another-block(cap: "square", radius: 8pt, adjacent: 0pt),
  another-block(cap: "square", radius: 8pt, adjacent: 1pt),

  [round],
  another-block(cap: "round", radius: 8pt, adjacent: none),
  another-block(cap: "round", radius: 8pt, adjacent: 0pt),
  another-block(cap: "round", radius: 8pt, adjacent: 1pt),
)

--- rect-cap-variations-radius-same-as-stroke-thickness paged ---
#import "block.typ": another-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [none], [0 width], [thin],
  [butt],
  another-block(cap: "butt", adjacent: none),
  another-block(cap: "butt", adjacent: 0pt),
  another-block(cap: "butt", adjacent: 1pt),

  [square],
  another-block(cap: "square", adjacent: none),
  another-block(cap: "square", adjacent: 0pt),
  another-block(cap: "square", adjacent: 1pt),

  [round],
  another-block(cap: "round", adjacent: none),
  another-block(cap: "round", adjacent: 0pt),
  another-block(cap: "round", adjacent: 1pt),
)

--- rect-cap-variations-radius-2-3-of-stroke-thickness paged ---
#import "block.typ": another-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [none], [0 width], [thin],
  [butt],
  another-block(cap: "butt", radius: 4pt, adjacent: none),
  another-block(cap: "butt", radius: 4pt, adjacent: 0pt),
  another-block(cap: "butt", radius: 4pt, adjacent: 1pt),

  [square],
  another-block(cap: "square", radius: 4pt, adjacent: none),
  another-block(cap: "square", radius: 4pt, adjacent: 0pt),
  another-block(cap: "square", radius: 4pt, adjacent: 1pt),

  [round],
  another-block(cap: "round", radius: 4pt, adjacent: none),
  another-block(cap: "round", radius: 4pt, adjacent: 0pt),
  another-block(cap: "round", radius: 4pt, adjacent: 1pt),
)

--- rect-cap-variations-radius-1-3-of-stroke-thickness paged ---
#import "block.typ": another-block
#set page(width: 7.5cm, margin: 0pt)
#table(
  stroke: none,
  columns: (0.75fr,) + 3 * (1fr,),
  [], [none], [0 width], [thin],
  [butt],
  another-block(cap: "butt", radius: 2pt, adjacent: none),
  another-block(cap: "butt", radius: 2pt, adjacent: 0pt),
  another-block(cap: "butt", radius: 2pt, adjacent: 1pt),

  [square],
  another-block(cap: "square", radius: 2pt, adjacent: none),
  another-block(cap: "square", radius: 2pt, adjacent: 0pt),
  another-block(cap: "square", radius: 2pt, adjacent: 1pt),

  [round],
  another-block(cap: "round", radius: 2pt, adjacent: none),
  another-block(cap: "round", radius: 2pt, adjacent: 0pt),
  another-block(cap: "round", radius: 2pt, adjacent: 1pt),
)

--- rect-adjacent-dashed-stroke-cap-change paged ---
#set page(width: auto)
#rect(
  height: 1.2cm,
  width: 1.5cm,
  stroke: (
    bottom: (cap: "square", thickness: 4pt, dash: "loosely-dashed"),
    left: (cap: "round", thickness: 4pt, dash: "loosely-dashed"),
  ),
)

--- rect-adjacent-dashed-stroke-thickness-change paged ---
#set page(width: auto)
#rect(
  height: 1.2cm,
  width: 1.5cm,
  stroke: (
    bottom: (thickness: 4pt, dash: "loosely-dashed"),
    left: (thickness: 8pt, dash: "loosely-dashed"),
  ),
)
//@@ file tests/suite/visualize_square.typ
// Test the `square` function.

--- square paged ---
// Default square.
#stack(
  dir: ltr,
  spacing: 0.5em,
  square(),
  square[hey!]
)

--- square-auto-sized paged ---
// Test auto-sized square.
#square(fill: eastern)[
  #set text(fill: white, weight: "bold")
  Typst
]

--- square-relatively-sized-child paged ---
// Test relative-sized child.
#square(fill: eastern)[
  #rect(width: 10pt, height: 5pt, fill: conifer)
  #rect(width: 40%, height: 5pt, stroke: conifer)
]

--- square-contents-overflow paged ---
// Test text overflowing height.
#set page(width: 75pt, height: 100pt)
#square(fill: conifer)[
  But, soft! what light through yonder window breaks?
]

--- square-height-limited paged ---
// Test that square does not overflow page.
#set page(width: 100pt, height: 75pt)
#square(fill: conifer)[
  But, soft! what light through yonder window breaks?
]

--- square-size-width-and-height eval ---
// Size wins over width and height.
// Error: 09-20 unexpected argument: width
#square(width: 10cm, height: 20cm, size: 1cm, fill: rgb("eb5278"))

--- square-relative-size paged ---
// Test relative width and height and size that is smaller
// than default size.
#set page(width: 120pt, height: 70pt)
#set align(bottom)
#let centered = align.with(center + horizon)
#stack(
  dir: ltr,
  spacing: 1fr,
  square(width: 50%, centered[A]),
  square(height: 50%),
  stack(
    square(size: 10pt),
    square(size: 20pt, centered[B])
  ),
)

--- square-circle-alignment paged ---
// Test alignment in automatically sized square and circle.
#set text(8pt)
#stack(
  dir: ltr,
  spacing: 0.5em,
  square(inset: 4pt)[
    Hey there, #align(center + bottom, rotate(180deg, [you!]))
  ],
  circle(align(center + horizon, [Hey.]))
)

--- square-circle-overspecified paged ---
// Test that minimum wins if both width and height are given.
#stack(
  dir: ltr,
  spacing: 2pt,
  square(width: 20pt, height: 40pt),
  circle(width: 20%, height: 40pt),
)

--- square-height-limited-stack paged ---
// Test square that is limited by region size.
#set page(width: 20pt, height: 10pt, margin: 0pt)
#stack(dir: ltr, square(fill: forest), square(fill: conifer))

--- square-no-overflow paged ---
// Test that square doesn't overflow due to its aspect ratio.
#set page(width: 40pt, height: 25pt, margin: 5pt)
#square()
#square[Hello there]

--- square-overflow-forced-width paged ---
// Test that a width-overflowing square is laid out regardless of the
// presence of inner content.
#set page(width: 60pt, height: 100pt)
#square(width: 150%)
#square(width: 150%)[Hello there]

--- square-overflow-forced-height paged ---
// Test that a height-overflowing square is laid out regardless of the
// presence of inner content.
#set page(width: 120pt, height: 60pt)
#square(height: 150%)
#square(height: 150%)[Hello there]

--- square-size-relative-invalid eval ---
// Size cannot be relative because we wouldn't know
// relative to which axis.
// Error: 15-18 expected length or auto, found ratio
#square(size: 50%)

--- square-rect-rounded paged ---
#set square(size: 20pt, stroke: 4pt)

// no radius for non-rounded corners
#stack(
  dir: ltr,
  square(),
  h(10pt),
  square(radius: 0pt),
  h(10pt),
  square(radius: -10pt),
)

#stack(
  dir: ltr,
  square(),
  h(10pt),
  square(radius: 0%),
  h(10pt),
  square(radius: -10%),
)

// small values for small radius
#stack(
  dir: ltr,
  square(radius: 1pt),
  h(10pt),
  square(radius: 5%),
  h(10pt),
  square(radius: 2pt),
)

// large values for large radius or circle
#stack(
  dir: ltr,
  square(radius: 8pt),
  h(10pt),
  square(radius: 10pt),
  h(10pt),
  square(radius: 12pt),
)

#stack(
  dir: ltr,
  square(radius: 45%),
  h(10pt),
  square(radius: 50%),
  h(10pt),
  square(radius: 55%),
)

--- square-base paged ---
// Test that square sets correct base for its content.
#set page(height: 80pt)
#square(width: 40%, rect(width: 60%, height: 80%))

--- square-size-beyond-default paged ---
// Test that setting a square's height beyond its default sizes it correctly.
#square()
#square(height: 60pt)
#square(width: 60pt)
#square(size: 60pt)
//@@ file tests/suite/visualize_stroke.typ
// Test lines.

--- stroke-constructor eval ---
// Converting to stroke
#test(stroke(red).paint, red)
#test(stroke(red).thickness, auto)
#test(stroke(2pt).paint, auto)
#test(stroke((cap: "round", paint: blue)).cap, "round")
#test(stroke((cap: auto, paint: blue)).cap, auto)
#test(stroke((cap: auto, paint: blue)).thickness, auto)

// Constructing with named arguments
#test(stroke(paint: blue, thickness: 8pt), 8pt + blue)
#test(stroke(thickness: 2pt), stroke(2pt))
#test(stroke(cap: "round").thickness, auto)
#test(stroke(cap: "round", thickness: auto).thickness, auto)

--- stroke-constructor-unknown-key eval ---
// Error: 9-21 unexpected key "foo", valid keys are "paint", "thickness", "cap", "join", "dash", and "miter-limit"
#stroke((foo: "bar"))

--- stroke-fields-simple eval ---
// Test stroke fields for simple strokes.
#test((1em + blue).paint, blue)
#test((1em + blue).thickness, 1em)
#test((1em + blue).cap, auto)
#test((1em + blue).join, auto)
#test((1em + blue).dash, auto)
#test((1em + blue).miter-limit, auto)

--- stroke-fields-complex eval ---
// Test complex stroke fields.
#let r1 = rect(stroke: (paint: cmyk(1%, 2%, 3%, 4%), thickness: 4em + 2pt, cap: "round", join: "bevel", miter-limit: 5.0, dash: none))
#let r2 = rect(stroke: (paint: cmyk(1%, 2%, 3%, 4%), thickness: 4em + 2pt, cap: "round", join: "bevel", miter-limit: 5.0, dash: (3pt, "dot", 4em)))
#let r3 = rect(stroke: (paint: cmyk(1%, 2%, 3%, 4%), thickness: 4em + 2pt, cap: "round", join: "bevel", dash: (array: (3pt, "dot", 4em), phase: 5em)))
#let s1 = r1.stroke
#let s2 = r2.stroke
#let s3 = r3.stroke
#test(s1.paint, cmyk(1%, 2%, 3%, 4%))
#test(s1.thickness, 4em + 2pt)
#test(s1.cap, "round")
#test(s1.join, "bevel")
#test(s1.miter-limit, 5.0)
#test(s3.miter-limit, auto)
#test(s1.dash, none)
#test(s2.dash, (array: (3pt, "dot", 4em), phase: 0pt))
#test(s3.dash, (array: (3pt, "dot", 4em), phase: 5em))

--- stroke-zero-thickness paged ---
// 0pt strokes must function exactly like 'none' strokes and not draw anything
#rect(width: 10pt, height: 10pt, stroke: none)
#rect(width: 10pt, height: 10pt, stroke: 0pt)
#rect(width: 10pt, height: 10pt, stroke: none, fill: blue)
#rect(width: 10pt, height: 10pt, stroke: 0pt + red, fill: blue)

#line(length: 30pt, stroke: 0pt)
#line(length: 30pt, stroke: (paint: red, thickness: 0pt, dash: ("dot", 1pt)))

#table(columns: 2, stroke: none)[A][B]
#table(columns: 2, stroke: 0pt)[A][B]

#curve(
  stroke: none,
  curve.move((0pt,  30pt)),
  curve.line((30pt, 30pt)),
  curve.line((15pt, 0pt)),
  curve.close()
)

#curve(
  stroke: 0pt,
  curve.move((0pt,  30pt)),
  curve.line((30pt, 30pt)),
  curve.line((15pt, 0pt)),
  curve.close()
)

--- stroke-text paged ---
#set text(size: 20pt)
#set page(width: auto)
#let v = [测试字体Test]

#text(stroke: 0.3pt + red, v)

#text(stroke: 0.7pt + red, v)

#text(stroke: 7pt + red, v)

#text(stroke: (paint: blue, thickness: 1pt, dash: "dashed"), v)

#text(stroke: 1pt + gradient.linear(..color.map.rainbow), v)

--- stroke-folding paged ---
// Test stroke folding.
#let sq(..args) = box(square(size: 10pt, ..args))

#set square(stroke: none)
#sq()
#set square(stroke: auto)
#sq()
#sq(fill: teal)
#sq(stroke: 2pt)
#sq(stroke: blue)
#sq(fill: teal, stroke: blue)
#sq(fill: teal, stroke: 2pt + blue)

--- stroke-composition paged ---
// Test stroke composition.
#set square(stroke: 4pt)
#set text(font: "Roboto")
#stack(
  dir: ltr,
  square(
    stroke: (left: red, top: yellow, right: green, bottom: blue),
    radius: 50%, align(center+horizon)[*G*],
    inset: 8pt
  ),
  h(0.5cm),
  square(
    stroke: (left: red, top: yellow + 8pt, right: green, bottom: blue + 2pt),
    radius: 50%, align(center+horizon)[*G*],
    inset: 8pt
  ),
  h(0.5cm),
  square(
    stroke: (left: red, top: yellow, right: green, bottom: blue),
    radius: 100%, align(center+horizon)[*G*],
    inset: 8pt
  ),
)

// Join between different solid strokes
#set square(size: 20pt, stroke: 2pt)
#set square(stroke: (left: green + 4pt, top: black + 2pt, right: blue, bottom: black + 2pt))
#stack(
  dir: ltr,
  square(),
  h(0.2cm),
  square(radius: (top-left: 0pt, rest: 1pt)),
  h(0.2cm),
  square(radius: (top-left: 0pt, rest: 8pt)),
  h(0.2cm),
	square(radius: (top-left: 0pt, rest: 100pt)),
)

// Join between solid and dotted strokes
#set square(stroke: (left: green + 4pt, top: black + 2pt, right: (paint: blue, dash: "dotted"), bottom: (paint: black, dash: "dotted")))
#stack(
  dir: ltr,
  square(),
  h(0.2cm),
  square(radius: (top-left: 0pt, rest: 1pt)),
  h(0.2cm),
  square(radius: (top-left: 0pt, rest: 8pt)),
  h(0.2cm),
	square(radius: (top-left: 0pt, rest: 100pt)),
)

--- stroke-repr eval ---
#let cases = (
  stroke(),
  stroke(2pt),
  stroke(red),
  stroke(2pt + red),
  stroke(cap: "round"),
  stroke(paint: blue, thickness: 2pt, dash: "dashed", miter-limit: 5.0)
)

// While repr isn't guaranteed to roundtrip with eval, we make an effort and if
// we cast with the `stroke` constructor, it should work for all cases.
#for case in cases {
  test(case, stroke(eval(repr(case))))
}

--- issue-3700-deformed-stroke paged ---
// Test shape fill & stroke for specific values that used to make the stroke
// deformed.
#rect(
  radius: 1mm,
  width: 100%,
  height: 10pt,
  stroke: (left: rgb("46b3c2") + 16.0mm),
)
//@@ file tests/suite/visualize_tiling.typ
// Test tilings.

--- tiling-line paged ---
// Tests that simple tilings work.
#set page(width: auto, height: auto, margin: 0pt)
#let t = tiling(size: (10pt, 10pt), line(stroke: 4pt, start: (0%, 0%), end: (100%, 100%)))
#rect(width: 50pt, height: 50pt, fill: t)

--- tiling-lines paged ---
#set page(width: auto, height: auto, margin: 0pt)

#let t = tiling(size: (10pt, 10pt), {
    place(line(stroke: 4pt, start: (0%, 0%), end: (100%, 100%)))
    place(line(stroke: 4pt, start: (100%,0%), end: (200%, 100%)))
    place(line(stroke: 4pt, start: (0%,100%), end: (100%, 200%)))
    place(line(stroke: 4pt, start: (-100%,0%), end: (0%, 100%)))
    place(line(stroke: 4pt, start: (0%,-100%), end: (100%, 0%)))
})
#rect(width: 50pt, height: 50pt, fill: t)

--- tiling-relative-self paged ---
// Test with relative set to `"self"`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #set line(stroke: green)
  #place(top + left, line(start: (0%, 0%), end: (100%, 100%), stroke: 1pt))
  #place(top + left, line(start: (0%, 100%), end: (100%, 0%), stroke: 1pt))
]

#set page(fill: t(), width: 100pt, height: 100pt)
#rect(
  width: 100%,
  height: 100%,
  fill: t(relative: "self"),
  stroke: 1pt + green,
)

--- tiling-relative-parent paged ---
// Test with relative set to `"parent"`
#let t(fill, ..args) = tiling(size: (30pt, 30pt), ..args)[
  #rect(width: 100%, height: 100%, fill: fill, stroke: none)
  #place(top + left, line(start: (0%, 0%), end: (100%, 100%), stroke: 1pt))
  #place(top + left, line(start: (0%, 100%), end: (100%, 0%), stroke: 1pt))
]

#set page(fill: t(white), width: 100pt, height: 100pt)

#rect(fill: t(none, relative: "parent"), width: 100%, height: 100%, stroke: 1pt)

--- tiling-relative-polygon paged ---
#polygon(
  fill: tiling(relative: "parent", circle(radius: 10pt)),
  stroke: blue,
  (20%, 0pt),
  (60%, 0pt),
  (80%, 20pt),
  (0%,  20pt),
)

--- tiling-relative-stack paged ---
#set rect(
  width: 100%,
  height: 20pt,
  fill: tiling(relative: "parent", circle(radius: 10pt)),
)
#stack(spacing: 5pt, rect(), rect())

--- tiling-small paged ---
// Tests small tilings for pixel accuracy.
#box(
  width: 8pt,
  height: 1pt,
  fill: tiling(size: (1pt, 1pt), square(size: 1pt, fill: black))
)
#v(-1em)
#box(
  width: 8pt,
  height: 1pt,
  fill: tiling(size: (2pt, 1pt), square(size: 1pt, fill: black))
)

--- tiling-zero-sized eval ---
// Error: 15-52 tile size must be non-zero
// Hint: 15-52 try setting the size manually
#line(stroke: tiling(curve(curve.move((1em, 0pt)))))

--- tiling-spacing-negative paged ---
// Test with spacing set to `(-10pt, -10pt)`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(spacing: (-10pt, -10pt)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-spacing-zero paged ---
// Test with spacing set to `(0pt, 0pt)`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(spacing: (0pt, 0pt)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-spacing-positive paged ---
// Test with spacing set to `(10pt, 10pt)`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(spacing: (10pt, 10pt,)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-offset-negative paged ---
// Test with offset set to `(-10pt, -10pt)`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(offset: (-10pt, -10pt)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-offset-zero paged ---
// Test with offset set to `(0pt, 0pt)`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(offset: (0pt, 0pt)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-offset-positive paged ---
// Test with offset set to `(10pt, 10pt)`
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(offset: (10pt, 10pt)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-offset-relative paged ---
// Test with offset set to `(10pt, 10pt)`
#let t(..args) = tiling(size: (30pt, 30pt), spacing: (10pt, 10pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(offset: (50%, -50%)), width: 100%, height: 100%, stroke: 1pt)

--- tiling-angle paged ---
// Test with 45deg
#let t(..args) = tiling(size: (30pt, 30pt), ..args)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(angle: 45deg), width: 100%, height: 100%, stroke: 1pt)

--- tiling-angle-with-offset paged ---
#let t(..args) = tiling(
  size: (30pt, 30pt),
  offset: (7.5pt * calc.sqrt(2), 15pt * calc.sqrt(2)),
  ..args,
)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(angle: 45deg), width: 100%, height: 100%, stroke: 1pt)

--- tiling-angle-with-relative-offset-and-spacing paged ---
#let t(..args) = tiling(
  size: (30pt, 30pt),
  spacing: (10pt, 5pt),
  offset: (50%, -50%),
  ..args,
)[
  #square(width: 100%, height: 100%, stroke: 1pt, fill: blue)
]

#set page(width: 100pt, height: 100pt)

#rect(fill: t(angle: -45deg), width: 100%, height: 100%, stroke: 1pt)

--- tiling-stroke paged ---
// Test tiling on strokes
#align(
  center + top,
  square(
    size: 50pt,
    fill: tiling(
      size: (5pt, 5pt),
      align(horizon + center, circle(fill: blue, radius: 2.5pt))
    ),
    stroke: 7.5pt + tiling(
      size: (5pt, 5pt),
      align(horizon + center, circle(fill: red, radius: 2.5pt))
    )
  )
)

--- tiling-stroke-relative-parent paged ---
// Test tiling on strokes with relative set to `"parent"`
// The tiling on the circle should align with the tiling on the square.
#align(
  center + top,
  block(
    width: 50pt,
    height: 50pt,
    fill: tiling(size: (5pt, 5pt), circle(radius: 2.5pt, fill: blue)),
    align(center + horizon, circle(
      radius: 15pt,
      stroke: 7.5pt + tiling(
        size: (5pt, 5pt), circle(radius: 2.5pt, fill: red), relative: "parent"
      ),
    ))
  )
)

--- tiling-text paged ---
// Test a tiling on some text. You shouldn't be able to see the text, if you can
// then that means that the transform matrices are not being applied to the text
// correctly.
#let t = tiling(
  size: (30pt, 30pt),
  relative: "parent",
  square(size: 30pt, fill: gradient.conic(..color.map.rainbow))
);

#set page(
  width: 140pt,
  height: 140pt,
  fill: t
)

#rotate(45deg, scale(x: 50%, y: 70%, rect(
  width: 100%,
  height: 100%,
  stroke: 1pt,
)[
  #lorem(10)

  #set text(fill: t)
  #lorem(10)
]))

--- tiling-text-offset-angle paged ---
// The text should be filled so that it seems invisible on the page.
#let t = tiling(
  size: (30pt, 30pt),
  offset: (7pt, 11pt),
  angle: 40deg,
  relative: "parent",
  square(size: 30pt, fill: gradient.conic(..color.map.rainbow)),
)

#set page(width: 140pt, height: 80pt, margin: 0pt, fill: t)
#set text(size: 28pt, fill: t)

OFFSET OFFSET

--- tiling-text-fill paged ---
#let t = tiling(
  size: (30pt, 30pt),
  relative: "parent",
  square(size: 30pt, fill: gradient.conic(..color.map.rainbow))
);
#set text(fill: t)

#lorem(20)

--- tiling-with-different-size paged ---
#let content = place(dx: 5pt, dy: 5pt, circle(radius: 5pt, fill: blue))
#let pat1 = tiling(size: (20pt, 20pt), content)
// Second tiling, only the size attribute changes
#let pat2 = tiling(size: (40pt, 20pt), content)

#rect(fill: pat1, width: 100pt, height: 20pt, stroke: 1pt)
#rect(fill: pat2, width: 100pt, height: 20pt, stroke: 1pt)

--- tiling-with-different-spacing paged ---
#let content = place(dx: 5pt, dy: 5pt, circle(radius: 5pt, fill: blue))
#let pat1 = tiling(size: (20pt, 20pt), content)
// Second tiling, only the spacing attribute changes
#let pat2 = tiling(size: (20pt, 20pt), spacing: (20pt, 0pt), content)

#rect(fill: pat1, width: 100pt, height: 20pt, stroke: 1pt)
#rect(fill: pat2, width: 100pt, height: 20pt, stroke: 1pt)

--- tiling-negative-area pdf ---
#rect(fill: tiling(size: (-2pt, -2pt))[Hello])
