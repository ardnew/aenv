# Grammar

This file serves as both specification and documentation for the language used by [`aenv`](https://github.com/ardnew/aenv):

```go

package "github.com/ardnew/aenv/lang"

```

1. [Introduction](#introduction) — describes the purpose and general characteristics of this language.
2. [Lexicon](#lexicon) — defines all first-class tokens emitted by the lexer.
3. [Syntax](#syntax) — defines how tokens are combined into meaningful data structures by the parser.

## Introduction

This language is designed to generate static environment variable definitions.

Environment variables are grouped into **namespaces**. Invoking a namespace produces the value of that namespace.

Identifiers placed between the namespace identifier and the value are called **parameters**. Parammeters are bound to the arguments provided when invoking the namespace.

### Terminology

- **Namespace**: The complete structure consisting of an identifier, optional parameters, and a value (e.g., `config x y : { ... }`).
- **Namespace identifier**: The identifier token that names the namespace (e.g., `config` in the example above).
- **Parameter**: An optional identifier token that modifies the behavior of a namespace (e.g., `x` and `y` in the example above).

## Lexicon

All input is consumed in units of UTF-8 code points. The grammar defined in this document describes how code point sequences construct tokens — first-class syntactical elements.

The grammar defined using a BNF-_ish_ syntax. Each rule consists of a rule identifier followed by a colon (`:`) and a single expression describing the code point sequences produced by that rule. Each rule is terminated by a semicolon (`;`).

Code point sequences may be grouped using bracketed expressions. Groups enable:

- association – to enforce precedence
- alternation – to list multiple alternative sequences
- repetition – summarized in the following table:

  |Bracketed expression|PCRE Equivalent|Meaning|
  |:------------------:|:-------------:|:-----:|
  |`( … )`|`( … )`|must occur once|
  |`[ … ]`|`( … )?`|may occur zero or one times|
  |`{ … }`|`( … )*`|may occur zero or more times|
  |`< … >`|`( … )+`|must occur one or more times|

```ebnf

identifier
  : (
      '[\p{L}\p{Nl}\p{Other_ID_Start}-\p{Pattern_Syntax}-\p{Pattern_White_Space}]' |
      '_'
    )
    {
      '[\p{L}\p{Nl}\p{Other_ID_Start}\p{Mn}\p{Mc}\p{Nd}\p{Pc}\p{Other_ID_Continue}-\p{Pattern_Syntax}-\p{Pattern_White_Space}]'
    }
    {
      any "-+.@/" <
        '[\p{L}\p{Nl}\p{Other_ID_Start}\p{Mn}\p{Mc}\p{Nd}\p{Pc}\p{Other_ID_Continue}-\p{Pattern_Syntax}-\p{Pattern_White_Space}]'
      >
    }
  ;

boolean_literal
  : ( 't' 'r' 'u' 'e' | 'f' 'a' 'l' 's' 'e' )
  ;

number_literal
  : [ '-' ] (
      (
        any "123456789" { number } [ '.' { number } ] |
        [ any "123456789" { number } ] '.' { number }
      )
        [ any "eE" [ any"+-" ] < number > ] |
      '0' {
        '.' { number }
          [ any "eE" [ any"+-" ] < number > ] |
        [ 'o' ] < any "01234567" > |
        'x' < any "0123456789abcdefABCDEF" > |
        'b' < any "01" >
      }
    )
  ;

string_literal
  : < '"' {
      not "\"\\" |
      '\\' any "\"\\abfnrtv" |
      '\\' any "ux" < any "0123456789abcdefABCDEF" > |
      '\\' < any "01234567" >
    } '"' >
  ;

expr_literal
  : '{' '{' { not "}\\" | any "}\\" not "}" } '}' '}'
  ;

delimiter
  : ','
  ;

separator
  : ';'
  ;

op_define
  : ':'
  ;

!line_comment
  : ( '/' '/' | '#' ) { not "\n" }
  ;

!block_comment
  : '/' '*' { not "*" | '*' not "/" } '*' '/'
  ;

```

## Syntax

The following rules combine lexer tokens into meaningful data structures.

Syntactical rules support `empty` productions, alternation, and recursion, but not repetition. The first rule is the parsing entry point.

```ebnf

Manifest
  : Namespaces
  ;

Namespaces
  : Namespace Namespaces
  | Namespace separator Namespaces
  | empty
  ;

Namespace
  : identifier Parameters op_define Value
  ;

Parameters
  : identifier Parameters
  | empty
  ;

Value
  : Literal
  | Tuple
  | Namespace
  | identifier
  ;

Values
  : Value
  | Value delimiter Values
  | empty
  ;

Literal
  : boolean_literal
  | number_literal
  | string_literal
  | expr_literal
  ;

Tuple
  : "{" Values "}"
  ;

```
