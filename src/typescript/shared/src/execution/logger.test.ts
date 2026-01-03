
import { logExecutionStart, logExecutionSuccess, logExecutionFailure, logExecutionPending } from './logger';
import { ExecutionService } from '../domain/services/execution';
import { ExecutionStatus } from '../types/pb/execution';
import { Logger } from 'winston';

// Mock ExecutionService
const mockCreate = jest.fn();
const mockUpdate = jest.fn();

const mockExecutionService = {
  create: mockCreate,
  update: mockUpdate
} as unknown as ExecutionService;

// Mock Logger
const mockLogger = {
  info: jest.fn(),
  error: jest.fn(),
  warn: jest.fn(),
  debug: jest.fn()
} as unknown as Logger;

// Mock Context
const mockCtx = {
  services: {
    execution: mockExecutionService
  },
  logger: mockLogger
};

describe('Execution Logger', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('logExecutionPending', () => {
    it('should log info and call service.create', async () => {
      const executionId = 'exec-123';
      const functionName = 'test-func';
      const trigger = 'http';

      await logExecutionPending(mockCtx, executionId, functionName, trigger);

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.stringContaining('Execution pending'),
        expect.anything()
      );
      expect(mockCreate).toHaveBeenCalledWith(executionId, expect.objectContaining({
        executionId,
        service: functionName,
        triggerType: trigger,
        status: ExecutionStatus.STATUS_PENDING
      }));
    });
  });

  describe('logExecutionStart', () => {
    it('should log info and call service.update', async () => {
      const executionId = 'exec-123';
      const trigger = 'http';
      const payload = { foo: 'bar' };

      await logExecutionStart(mockCtx, executionId, trigger, payload);

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.stringContaining('Execution started'),
        expect.anything()
      );
      expect(mockUpdate).toHaveBeenCalledWith(executionId, expect.objectContaining({
        status: ExecutionStatus.STATUS_STARTED,
        inputsJson: JSON.stringify(payload)
      }));
    });
  });

  describe('logExecutionSuccess', () => {
    it('should log info and call service.update', async () => {
      const executionId = 'exec-123';
      const result = { success: true };

      await logExecutionSuccess(mockCtx, executionId, result);

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.stringContaining('Execution completed'),
        expect.anything()
      );
      expect(mockUpdate).toHaveBeenCalledWith(executionId, expect.objectContaining({
        status: ExecutionStatus.STATUS_SUCCESS,
        outputsJson: JSON.stringify(result)
      }));
    });
  });

  describe('logExecutionFailure', () => {
    it('should log error and call service.update', async () => {
      const executionId = 'exec-123';
      const error = new Error('Test Error');

      await logExecutionFailure(mockCtx, executionId, error);

      expect(mockLogger.error).toHaveBeenCalledWith(
        expect.stringContaining('Execution failed'),
        expect.anything()
      );
      expect(mockUpdate).toHaveBeenCalledWith(executionId, expect.objectContaining({
        status: ExecutionStatus.STATUS_FAILED,
        errorMessage: 'Test Error'
      }));
    });
  });
});
