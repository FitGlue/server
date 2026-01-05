import { FrameworkContext } from './index';

export interface AuthResult {
    userId: string;
    scopes: string[];
}

export interface AuthStrategy {
    name: string;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null>;
}
