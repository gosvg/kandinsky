/*

viz is a simple demo of kandinsky in action. On launch, viz will listen on
the given port (default :6000, configurable with -http) and provide two endpoints:

/viz: Used for visualizing primitive types. Define a type (int, float, bool) with the type
query parameter and pass the appropriate value with the v query parameter.

/struct: A demonstration of a data structure as rendered by kandinsky.
*/
package main // import "github.com/gosvg/kandinsky/cmd/viz"
