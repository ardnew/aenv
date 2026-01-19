# Grammar

```ebnf

package "github.com/ardnew/envcomp/pkg/lang/internal"

```

This file serves as both definition and documentation for the language used by [`envcomp`](https://github.com/ardnew/envcomp).

1. [Lexical grammar](#lexical-grammar) — defines the first-class tokens emitted by the lexer.
2. [Syntactic grammar](#syntactic-grammar) — defines how tokens are combined to form semantics.

## Lexical Grammar

All input is consumed in units of UTF-8 code points. The grammar defined in this document describes how code point sequences construct tokens — first-class syntactical elements.

The grammar is expressed similar to ABNF, where each rule consists of a rule identifier followed by a colon (`:`) and a single expression describing the code point sequences produced by that rule. Each rule is terminated by a semicolon (`;`).

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
   : ( '[\p{L}\p{Nl}\p{Other_ID_Start}-\p{Pattern_Syntax}-\p{Pattern_White_Space}]' | '_' )
     {
       '[\p{L}\p{Nl}\p{Other_ID_Start}\p{Mn}\p{Mc}\p{Nd}\p{Pc}\p{Other_ID_Continue}-\p{Pattern_Syntax}-\p{Pattern_White_Space}]'
     }
     {
       any "-+.@/"
       < '[\p{L}\p{Nl}\p{Other_ID_Start}\p{Mn}\p{Mc}\p{Nd}\p{Pc}\p{Other_ID_Continue}-\p{Pattern_Syntax}-\p{Pattern_White_Space}]' >
     }
   ;

boolean_literal
   : ( 't' 'r' 'u' 'e' | 'f' 'a' 'l' 's' 'e' )
   ;

number_literal
   : [ '-' ] (
      any "123456789" { number } [ '.' { number } ]
         [ any "eE" [ any"+-" ] < number > ] |
      '0' {
         '.' { number }
            [ any "eE" [ any"+-" ] < number > ] |
         < any "01234567" > |
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

---

## Syntactic Grammar

Tokens produced by the lexer are combined according to the following rules to construct meaningful data structures.

The first syntactic rule serves as the entry point for parsing.

```ebnf

Manifest
   : Definitions
   ;

Definitions
   : Definition Definitions
   | Definition separator Definitions
   | empty
   ;

Definition
   : identifier Parameters op_define Value
   ;

Parameters
   : Value Parameters
   | empty
   ;

Value
   : Literal
   | Tuple
   | Definition
   | identifier
   ;

Literal
   : boolean_literal
   | number_literal
   | string_literal
   | expr_literal
   ;

Tuple
   : "{" Aggregate "}"
   ;

Aggregate
   : Value
   | Value delimiter Aggregate
   | empty
   ;

```
