# Preto

`preto` is a toy language which trasnpiles to Protocol Buffers.

**Goals**

- Provide a simplified, identation-based syntax
- Make it easier to add desirable features such as composing
  protobuf messages from multiple other messages
- Learn to hack together a basic compiler


**Example**


```
package example

option java_package "java_pkg_name"

msg MyMessage
  foo str 1
  bar int 2      [deprecated]
  complex int 99 [foo_options.opt1=123,foo_options.opt2="baz"]

  # I am a comment
  bob bytes 8
  foo map[str]int 4
  bar []int 3

  # whoa I am nested message
  msg NestedMessage
    str sound 1

  enum TheEnum
    ONE 1
    THREE 3
    TWO 2

  oneof something
    first_thing     str 1
    or_second_thing str 3
```
