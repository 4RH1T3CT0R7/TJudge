#!/usr/bin/env python3
from __future__ import annotations

import asyncio
import logging

from napominayka.app import create_bot, create_dispatcher, setup_bot_commands
from napominayka.config import load_settings
from napominayka.db.session import init_db
from napominayka.services.scheduler import init_scheduler
from napominayka.services.jobs import (
    daily_cleanup_job,
    labs_evening_reminder_job,
    labs_finalize_job,
)
from napominayka.services.reminders import send_reminder_job
from napominayka.services.lab_pings import labs_morning_ping_job, labs_morning_ping_scan


async def main() -> None:
    logging.basicConfig(level=logging.INFO)

    settings = load_settings()
    await init_db(settings.database_url)
    await init_scheduler(
        send_reminder_job=send_reminder_job,
        daily_cleanup_job=daily_cleanup_job,
        labs_morning_ping_scan=labs_morning_ping_scan,
        labs_finalize_job=labs_finalize_job,
        labs_evening_reminder_job=labs_evening_reminder_job,
        labs_morning_ping_job=labs_morning_ping_job,
    )

    bot = create_bot(settings)
    dp = create_dispatcher()
    await setup_bot_commands(bot)

    polling_task = asyncio.create_task(
        dp.start_polling(bot, allowed_updates=dp.resolve_used_update_types()),
        name="bot_polling",
    )

    done, pending = await asyncio.wait({polling_task}, return_when=asyncio.FIRST_EXCEPTION)

    for task in pending:
        task.cancel()
        try:
            await task
        except asyncio.CancelledError:
            pass

    for task in done:
        exc = task.exception()
        if exc:
            raise exc


if __name__ == "__main__":
    asyncio.run(main())
