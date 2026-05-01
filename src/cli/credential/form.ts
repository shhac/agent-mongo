import {
  type Category,
  type Field,
  type Spec,
  classifyError,
  getDefault,
} from "../../lib/dialog/index.ts";

/**
 * promptMissingViaDialog asks the user (via a native OS dialog) for any
 * secret fields not supplied by --username / --password. Returns the
 * potentially filled-in values along with a non-null error envelope on
 * dialog failure.
 *
 * The LLM driving the CLI never sees what the user types — input goes
 * directly into the OS dialog, and only the call return makes it back
 * into agent-mongo's process memory.
 */
export type FormError = {
  message: string;
  fixableBy: Category;
  hint: string;
};

export type FormResult = {
  username: string;
  password: string;
  error?: FormError;
};

export async function promptMissingViaDialog(input: {
  name: string;
  username: string;
  password: string;
}): Promise<FormResult> {
  const { spec, slots } = buildCredentialSpec(input);
  if (spec.items.length === 0) {
    return { username: input.username, password: input.password };
  }

  const prompter = getDefault();
  const availabilityErr = prompter.available();
  if (availabilityErr) {
    return {
      username: input.username,
      password: input.password,
      error: classifyDialogErr(availabilityErr, input.name),
    };
  }

  try {
    const results = await prompter.prompt(spec);
    const out = applyResults({
      username: input.username,
      password: input.password,
      results,
      slots,
    });
    return { username: out.username, password: out.password };
  } catch (err) {
    return {
      username: input.username,
      password: input.password,
      error: classifyDialogErr(err, input.name),
    };
  }
}

/**
 * Pairs a dialog Field with the credential field it should fill. Keeping
 * them adjacent (built once, consumed once) removes string-coupling between
 * spec construction and result folding.
 */
type FieldSlot = {
  field: Field;
  destKey: "username" | "password";
};

/**
 * Assembles the dialog Spec for any blank credential fields. The returned
 * slots have the same length and order as spec.items; applyResults walks
 * them in lockstep.
 */
export function buildCredentialSpec(input: { name: string; username: string; password: string }): {
  spec: Spec;
  slots: FieldSlot[];
} {
  const candidates: { slot: FieldSlot; current: string }[] = [
    {
      slot: {
        field: { id: "username", label: "Database username", inputType: "text" },
        destKey: "username",
      },
      current: input.username,
    },
    {
      slot: {
        field: { id: "password", label: "Database password", inputType: "password" },
        destKey: "password",
      },
      current: input.password,
    },
  ];
  const slots: FieldSlot[] = [];
  const items: Field[] = [];
  for (const c of candidates) {
    if (c.current !== "") {
      continue;
    }
    slots.push(c.slot);
    items.push(c.slot.field);
  }
  return {
    spec: { title: `agent-mongo credential: ${input.name}`, items },
    slots,
  };
}

/**
 * Writes each Result's value into the matching slot's destination by ID.
 * Order is preserved, but matching by ID is safer against future spec
 * rearrangement.
 */
export function applyResults(input: {
  username: string;
  password: string;
  results: { id: string; value: string }[];
  slots: FieldSlot[];
}): { username: string; password: string } {
  const byId = new Map<string, FieldSlot["destKey"]>();
  for (const s of input.slots) {
    byId.set(s.field.id, s.destKey);
  }
  let { username, password } = input;
  for (const r of input.results) {
    const key = byId.get(r.id);
    if (key === "username") {
      username = r.value;
    } else if (key === "password") {
      password = r.value;
    }
  }
  return { username, password };
}

/**
 * Adapts a dialog package error to agent-mongo's form-error envelope.
 * The sentinel→category mapping lives in dialog.classifyError so the
 * mapping itself never drifts; this function only augments the hint with
 * agent-mongo-specific guidance.
 */
function classifyDialogErr(err: unknown, name: string): FormError {
  const [cat, baseHint] = classifyError(err);
  let hint = baseHint;
  if (cat === "human") {
    hint =
      "agent-mongo credential add --form requires a graphical desktop session. " +
      "Ask the user to run on their local machine, or fall back to non-interactive: " +
      `agent-mongo credential add ${name} --username <u> --password <secret>`;
  } else if (cat === "retry") {
    hint = "User cancelled the dialog. Re-run agent-mongo credential add --form to retry.";
  }
  return {
    message: err instanceof Error ? err.message : String(err),
    fixableBy: cat,
    hint,
  };
}
