/**
 * Keyless model for the real-CLI run: loads one env skill through the `skill`
 * tool, then answers with what came back. It also reports a leak if a skill hev
 * withheld ever appears in a request, so the gating failure is visible on stdout.
 */
import type { Context } from '@deepseek-ai/cordis'
import {
  CallId,
  LlmAdapter,
  ReasoningEffortId,
  type GenerateOptions,
  type LlmResolvedModelInfo,
  type StreamChunk,
} from '@deepseek-ai/dsh-llm'

const OFF = ReasoningEffortId('off')
const HIGH = ReasoningEffortId('high')
const WITHHELD = 'secret-skill'

class HevMockAdapter extends LlmAdapter {
  private requests = 0

  override async resolveModel(provider: string, model: string): Promise<LlmResolvedModelInfo> {
    return {
      provider,
      id: model,
      name: model,
      reasoning: { efforts: [{ id: OFF, name: 'Off' }, { id: HIGH, name: 'High' }], defaultEffort: HIGH },
    }
  }

  async * stream(options: GenerateOptions): AsyncIterable<StreamChunk> {
    this.requests += 1
    const transcript = JSON.stringify(options.messages)
    if (transcript.includes(WITHHELD)) {
      yield * say(`HEV_LEAK: ${WITHHELD} reached the model`)
      return
    }
    // Scan the whole transcript, not just the last message: loading a skill can
    // append a catalog-update message after the tool result, and keying on the
    // last message alone would make this adapter re-issue the call forever.
    const toolResult = options.messages
      .flatMap(message => message.content)
      .find(block => block.type === 'tool-result')
    if (toolResult === undefined && this.requests < 3) {
      const args = JSON.stringify({ name: 'code-review' })
      yield { type: 'block-start', index: 0, blockType: 'tool-call' }
      yield { type: 'tool-call-delta', index: 0, id: CallId('hev-skill-call'), name: 'skill', argumentsDelta: args }
      yield { type: 'block-end', index: 0, block: { type: 'tool-call', id: CallId('hev-skill-call'), name: 'skill', arguments: args } }
      yield { type: 'finish', reason: { kind: 'tool-calls' } }
      return
    }
    const body = (toolResult?.content ?? [])
      .filter(block => block.type === 'text')
      .map(block => block.text)
      .join('')
    const marker = /<skill_instructions>\n(.+)\n<\/skill_instructions>/u.exec(body)?.[1] ?? 'NO_BODY'
    yield * say(`HEV_OK: ${marker}`)
  }
}

function * say(text: string): Generator<StreamChunk> {
  yield { type: 'block-start', index: 0, blockType: 'text' }
  yield { type: 'text-delta', index: 0, text }
  yield { type: 'block-end', index: 0, block: { type: 'text', text } }
  yield { type: 'finish', reason: { kind: 'stop' } }
}

/** Cordis plugin name. */
export const name = 'hev-mock-llm'
/** Service this keyless adapter registers into. */
export const inject = ['llm']

/** Register the keyless adapter on the `cli-mock` route the overlay selects. */
export function apply(ctx: Context): void {
  ctx.effect(function* () {
    yield ctx.llm.registerAdapter(['cli-mock'], new HevMockAdapter())
  }, 'hev mock llm')
}
