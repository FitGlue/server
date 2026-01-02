import { FrameworkContext } from './index';

export interface AuthResult {
    userId: string;
    scopes: string[];
}

export interface AuthStrategy {
    name: string;
    authenticate(req: any, ctx: FrameworkContext): Promise<AuthResult | null>;
}
