import { Command, ExactArgs, ref } from "vipvot";
import { storeCredential } from "../../lib/config.ts";
import { printError, printErrorObject, printJsonRaw } from "../../lib/output.ts";
import { FormError, promptMissingViaDialog } from "./form.ts";

type AddOpts = {
  username: string;
  password: string;
  form: boolean;
};

export function buildAddCommand(): Command {
  const username = ref<string>("");
  const password = ref<string>("");
  const form = ref<boolean>(false);

  const cmd = Command({
    use: "add <name>",
    short: "Add or update a stored credential",
    args: ExactArgs(1),
    run: async (_cmd, args) => {
      const [name] = args as [string];
      try {
        const resolved = await resolveCredentials(name, {
          username: username.value,
          password: password.value,
          form: form.value,
        });
        if (!resolved) {
          return;
        }
        const { storage } = storeCredential(name, resolved);
        printJsonRaw({
          ok: true,
          credential: name,
          username: resolved.username,
          storage,
          hint: `Use with: agent-mongo connection add <alias> <uri> --credential ${name}`,
        });
      } catch (err) {
        if (err instanceof FormError) {
          printErrorObject({
            error: err.message,
            fixableBy: err.fixableBy,
            hint: err.hint,
            credential: name,
          });
          return;
        }
        printError(err instanceof Error ? err.message : "Failed to add credential");
      }
    },
  });

  cmd.flags().stringVarP(username, "username", "", "", "MongoDB username");
  cmd.flags().stringVarP(password, "password", "", "", "MongoDB password");
  cmd
    .flags()
    .boolVarP(
      form,
      "form",
      "",
      false,
      "Prompt for missing username/password via a native OS dialog (LLM-safe; the secret is typed directly into the OS, never seen by the agent)",
    );

  return cmd;
}

async function resolveCredentials(
  name: string,
  opts: AddOpts,
): Promise<{ username: string; password: string } | null> {
  const { username, password } = opts.form
    ? await promptMissingViaDialog({
        name,
        username: opts.username,
        password: opts.password,
      })
    : { username: opts.username, password: opts.password };

  if (!username || !password) {
    printError(
      "Missing --username and/or --password. Pass them on the command line, or use --form for an OS dialog.",
    );
    return null;
  }
  return { username, password };
}
