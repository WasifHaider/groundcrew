import { Module } from '@nestjs/common';
import { HealthController } from './health/health.controller.js';

@Module({
  imports: [],
  controllers: [HealthController],
})
export class AppModule {}
