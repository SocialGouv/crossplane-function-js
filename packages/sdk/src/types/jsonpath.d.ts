declare module "jsonpath" {
  export function value(obj: any, path: string): any
  export function query(obj: any, path: string): any[]
  export function paths(obj: any, path: string): string[][]
  export function nodes(obj: any, path: string): Array<{ path: string[]; value: any }>
  export function parent(obj: any, path: string): any
  export function apply(obj: any, path: string, fn: (value: any) => any): any
  export function stringify(path: string[]): string
  export function parse(pathExpression: string): string[]
}
