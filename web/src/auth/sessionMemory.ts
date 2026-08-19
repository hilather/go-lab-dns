let csrf = ''

export function getCsrf(): string {
  return csrf
}

export function setCsrf(token: string): void {
  csrf = token
}

export function clear(): void {
  csrf = ''
}
