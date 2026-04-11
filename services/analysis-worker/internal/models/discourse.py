from pydantic import BaseModel
from typing import Optional


class ProbeEticTag(BaseModel):
    """
    Etic (observer-imposed) classification of a source's discourse function.

    Defined by WP-001 §4.2. The four discourse functions are:
    - epistemic_authority: norm-setting through informational baseline
    - power_legitimation: institutional agenda-setting and framing
    - cohesion_identity: community cohesion and identity production
    - subversion_friction: counter-hegemonic discourse and resistance

    ``function_weights`` are intentionally optional — quantifying the relative
    strength of discourse functions requires the formalized classification
    process (WP-001 §4.4, Steps 1-2: area expert nomination and peer review).
    """
    primary_function: str
    secondary_function: Optional[str] = None
    function_weights: Optional[dict[str, float]] = None


class ProbeEmicTag(BaseModel):
    """
    Emic (participant-perspective) designation of a source.

    Defined by WP-001 §4.2. Captures how the source identifies itself
    or is identified within its own cultural context. The emic designation
    preserves the local name, and emic_context documents the source's
    self-understanding and structural position.
    """
    emic_designation: str
    emic_context: str
    emic_language: str


class DiscourseContext(BaseModel):
    """
    Propagation model for discourse classification in SilverMeta.

    Defined by WP-001 §7.2. A lightweight subset of the full etic/emic
    classification that travels with each document through the pipeline.
    Populated by source adapters from the ``source_classifications`` table.
    """
    primary_function: str
    secondary_function: Optional[str] = None
    emic_designation: str
