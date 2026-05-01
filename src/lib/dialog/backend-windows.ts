import { type PromptCtx, runAndDecode } from "./run.ts";

/**
 * Windows backend: uses `Get-Credential` for passwords (native masked
 * dialog) and a VisualBasic `InputBox` for plain text. Both run via
 * PowerShell. Get-Credential needs a non-empty UserName, so we pass a
 * placeholder and only return the password.
 */
export async function promptWindows(ctx: PromptCtx): Promise<string> {
  const label = escapePowerShell(ctx.item.label);
  const title = escapePowerShell(ctx.title);
  const script =
    ctx.item.inputType === "password"
      ? `$ErrorActionPreference='Stop';$c=Get-Credential -Message '${label}' -UserName 'user';if(-not $c){exit 1};$c.GetNetworkCredential().Password`
      : `$ErrorActionPreference='Stop';Add-Type -AssemblyName Microsoft.VisualBasic;$v=[Microsoft.VisualBasic.Interaction]::InputBox('${label}','${title}','');if($v -eq ''){exit 1};$v`;
  return runAndDecode({ argv: ["powershell", "-NoProfile", "-Command", script], ctx });
}

/**
 * PowerShell single-quoted string literal escaping: only `'` needs to
 * become `''`. `$()`, backticks, and other metacharacters are inert
 * inside single-quoted strings.
 */
export function escapePowerShell(s: string): string {
  return s.replace(/'/g, "''");
}
