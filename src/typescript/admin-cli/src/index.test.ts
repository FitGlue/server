import { addActivitiesCommands } from './commands/activities';
import { Command } from 'commander';

describe('Admin CLI Sanity Check', () => {
  it('should pass a basic truthy test', () => {
    expect(true).toBe(true);
  });

  // TODO: Refactor commands to be testable (exported functions)
  // currently index.ts runs side-effects on import.
});

describe('Admin CLI: Activities Commands', () => {
  let program: Command;

  beforeEach(() => {
    program = new Command();
  });

  it('should register activities commands', () => {
    addActivitiesCommands(program);
    const commands = program.commands.map(c => c.name());
    expect(commands).toContain('activities:list-processed');
    // Expect other commands if they exist, but at least verify registration happening
  });
});
